package main

import (
	"bufio"
	"encoding/binary"
	"gopkg.in/redis.v3"
	"io"
	"log"
	"os"
	"syscall"
	"time"
	"bytes"
)

type Cache struct {
	data         map[string]bool
	newData      map[string]bool
	ts           []byte
	path         string
	useRedis     bool
	redisClient  *redis.Client
	redisOptions redis.Options
}

func (c *Cache) OpenCache() (err error) {
	var key string

	tsBuf := bytes.NewBuffer(nil)
	binary.Write(tsBuf, binary.BigEndian, time.Now().Unix())

	c.data = make(map[string]bool)
	c.newData = make(map[string]bool)
	c.ts = tsBuf.Bytes()

	if c.useRedis {
		c.redisClient = redis.NewClient(&c.redisOptions)
	}

	cacheFile, err := os.Open(c.path)
	if err != nil && !os.IsNotExist(err) {
		return err
	} else if os.IsNotExist(err) {
		return nil
	}

	reader := bufio.NewReader(cacheFile)
	for err != io.EOF {
		if key, err = reader.ReadString('\n'); err != nil && err != io.EOF {
			return err
		}
		if key != "" && key != "" {
			key = key[:len(key)-1]
			c.data[key] = true
		}
	}

	return nil
}

func (c *Cache) Getset(id, host, msgId string) bool {
	if c.useRedis {
		res := c.redisClient.HExists("ua:"+host, id)
		if res.Err() != nil && res.Err() != redis.Nil {
			log.Fatalf("Error using redis cache: %s", res.Err())
		}

		present := res.Val()

		res = c.redisClient.HSet("ua:"+host, id, string(c.ts))
		if res.Err() != nil && res.Err() != redis.Nil {
			log.Fatalf("Error using redis cache: %s", res.Err())
		}

		return present
	} else {
		if _, has := c.data[msgId]; has {
			return true
		}
		if _, has := c.newData[msgId]; has {
			return true
		}
		c.newData[msgId] = true
	}
	return false
}

func (c *Cache) Dump() error {
	if c.useRedis {
		return nil
	}

	cacheFile, err := os.OpenFile(c.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0660)
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	if err = syscall.Flock(int(cacheFile.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}

	writer := bufio.NewWriter(cacheFile)
	for key, _ := range c.newData {
		if _, err = writer.WriteString(key); err != nil {
			return err
		}
		if _, err = writer.WriteString("\n"); err != nil {
			return err
		}
	}
	if err = writer.Flush(); err != nil {
		return err
	}

	return nil
}
