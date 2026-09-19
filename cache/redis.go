package cache

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RedisClient struct {
	addr    string
	timeout time.Duration
	pool    *sync.Pool
}

func NewRedisClient(addr string) *RedisClient {
	if addr == "" {
		addr = "redis:6379"
	}
	rc := &RedisClient{
		addr:    addr,
		timeout: 2 * time.Second,
	}
	rc.pool = &sync.Pool{
		New: func() interface{} {
			conn, err := net.DialTimeout("tcp", rc.addr, rc.timeout)
			if err != nil {
				return nil
			}
			return conn
		},
	}
	return rc
}

func (rc *RedisClient) getConn() (net.Conn, error) {
	conn := rc.pool.Get()
	if conn == nil {
		return net.DialTimeout("tcp", rc.addr, rc.timeout)
	}
	return conn.(net.Conn), nil
}

func (rc *RedisClient) putConn(conn net.Conn) {
	if conn != nil {
		rc.pool.Put(conn)
	}
}

func (rc *RedisClient) Ping() error {
	conn, err := rc.getConn()
	if err != nil {
		return err
	}
	defer rc.putConn(conn)

	_ = conn.SetDeadline(time.Now().Add(rc.timeout))
	_, err = conn.Write([]byte("*1\r\n$4\r\nPING\r\n"))
	if err != nil {
		conn.Close()
		return err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return err
	}

	if !strings.HasPrefix(line, "+PONG") {
		return fmt.Errorf("unexpected redis ping response: %s", strings.TrimSpace(line))
	}
	return nil
}

func (rc *RedisClient) Set(key, value string, ttlSeconds int) error {
	conn, err := rc.getConn()
	if err != nil {
		return err
	}
	defer rc.putConn(conn)

	_ = conn.SetDeadline(time.Now().Add(rc.timeout))
	var cmd string
	if ttlSeconds > 0 {
		cmd = fmt.Sprintf("*5\r\n$3\r\nSET\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n$2\r\nEX\r\n$%d\r\n%d\r\n",
			len(key), key, len(value), value, len(strconv.Itoa(ttlSeconds)), ttlSeconds)
	} else {
		cmd = fmt.Sprintf("*3\r\n$3\r\nSET\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n",
			len(key), key, len(value), value)
	}

	if _, err := conn.Write([]byte(cmd)); err != nil {
		conn.Close()
		return err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return err
	}

	if !strings.HasPrefix(line, "+OK") {
		return fmt.Errorf("redis set error: %s", strings.TrimSpace(line))
	}
	return nil
}

func (rc *RedisClient) Exists(key string) (bool, error) {
	conn, err := rc.getConn()
	if err != nil {
		return false, err
	}
	defer rc.putConn(conn)

	_ = conn.SetDeadline(time.Now().Add(rc.timeout))
	cmd := fmt.Sprintf("*2\r\n$6\r\nEXISTS\r\n$%d\r\n%s\r\n", len(key), key)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		conn.Close()
		return false, err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return false, err
	}

	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, ":1") {
		return true, nil
	}
	return false, nil
}

func (rc *RedisClient) Get(key string) (string, error) {
	conn, err := rc.getConn()
	if err != nil {
		return "", err
	}
	defer rc.putConn(conn)

	_ = conn.SetDeadline(time.Now().Add(rc.timeout))
	cmd := fmt.Sprintf("*2\r\n$3\r\nGET\r\n$%d\r\n%s\r\n", len(key), key)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		conn.Close()
		return "", err
	}

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return "", err
	}

	line = strings.TrimSpace(line)
	if line == "$-1" {
		return "", nil // key does not exist
	}

	if strings.HasPrefix(line, "$") {
		length, err := strconv.Atoi(line[1:])
		if err != nil {
			return "", err
		}
		buf := make([]byte, length+2) // +2 for \r\n
		n := 0
		for n < length+2 {
			readN, err := reader.Read(buf[n:])
			if err != nil {
				conn.Close()
				return "", err
			}
			n += readN
		}
		return string(buf[:length]), nil
	}

	return "", fmt.Errorf("redis get unexpected response: %s", line)
}

func (rc *RedisClient) Del(key string) error {
	conn, err := rc.getConn()
	if err != nil {
		return err
	}
	defer rc.putConn(conn)

	_ = conn.SetDeadline(time.Now().Add(rc.timeout))
	cmd := fmt.Sprintf("*2\r\n$3\r\nDEL\r\n$%d\r\n%s\r\n", len(key), key)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		conn.Close()
		return err
	}

	reader := bufio.NewReader(conn)
	_, err = reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return err
	}
	return nil
}

// User & Token Revocation Helpers

func (rc *RedisClient) RevokeUser(userID string, ttlSeconds int) error {
	if ttlSeconds <= 0 {
		ttlSeconds = 86400 // Default 24 hours
	}
	return rc.Set("auth:revoked:user:"+userID, "revoked", ttlSeconds)
}

func (rc *RedisClient) IsUserRevoked(userID string) (bool, error) {
	if userID == "" {
		return false, nil
	}
	return rc.Exists("auth:revoked:user:" + userID)
}

func (rc *RedisClient) RevokeUserBefore(userID string, timestamp int64, ttlSeconds int) error {
	if ttlSeconds <= 0 {
		ttlSeconds = 86400 * 30 // 30 days
	}
	return rc.Set("auth:revoked_before:"+userID, strconv.FormatInt(timestamp, 10), ttlSeconds)
}

func (rc *RedisClient) GetRevokedBefore(userID string) (int64, error) {
	if userID == "" {
		return 0, nil
	}
	val, err := rc.Get("auth:revoked_before:" + userID)
	if err != nil || val == "" {
		return 0, err
	}
	ts, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, err
	}
	return ts, nil
}

func (rc *RedisClient) RevokeToken(tokenID string, ttlSeconds int) error {
	if ttlSeconds <= 0 {
		ttlSeconds = 86400
	}
	return rc.Set("auth:revoked:token:"+tokenID, "revoked", ttlSeconds)
}

func (rc *RedisClient) IsTokenRevoked(tokenID string) (bool, error) {
	if tokenID == "" {
		return false, nil
	}
	return rc.Exists("auth:revoked:token:" + tokenID)
}
