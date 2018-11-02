// Copyright 2018 Burak Sezer
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"context"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buraksezer/olric"
)

var testConfig = &Config{
	DialTimeout: time.Second,
	KeepAlive:   time.Second,
	MaxConn:     100,
}

func getFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func newOlric() (*olric.Olric, chan struct{}, error) {
	port, err := getFreePort()
	if err != nil {
		return nil, nil, err
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)
	cfg := &olric.Config{Name: addr}
	db, err := olric.New(cfg)
	if err != nil {
		return nil, nil, err
	}

	done := make(chan struct{})
	go func() {
		rerr := db.Start()
		if rerr != nil {
			log.Printf("[ERROR] Expected nil. Got %v", rerr)
		}
		close(done)
	}()
	time.Sleep(100 * time.Millisecond)
	testConfig.Addrs = []string{addr}
	return db, done, nil
}

func TestClient_Get(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		serr := db.Shutdown(context.Background())
		if serr != nil {
			t.Errorf("Expected nil. Got %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	name := "mymap"
	dm := db.NewDMap(name)
	key, value := "my-key", "my-value"
	err = dm.Put(key, value)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}
	val, err := c.NewDMap(name).Get(key)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}
	if val.(string) != value {
		t.Fatalf("Expected value %s. Got: %s", val.(string), value)
	}
}

func TestClient_Put(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		serr := db.Shutdown(context.Background())
		if serr != nil {
			t.Errorf("Expected nil. Got %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	name := "mymap"
	key, value := "my-key", "my-value"
	err = c.NewDMap(name).Put(key, value)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	dm := db.NewDMap(name)
	val, err := dm.Get(key)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}
	if val.(string) != value {
		t.Fatalf("Expected value %s. Got: %s", val.(string), value)
	}
}

func TestClient_PutEx(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		serr := db.Shutdown(context.Background())
		if serr != nil {
			t.Errorf("Expected nil. Got %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	name := "mymap"
	key, value := "my-key", "my-value"
	err = c.NewDMap(name).PutEx(key, value, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	dm := db.NewDMap(name)
	_, err = dm.Get(key)
	if err != olric.ErrKeyNotFound {
		t.Fatalf("Expected nil. Got: %v", err)
	}
}

func TestClient_Delete(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		serr := db.Shutdown(context.Background())
		if serr != nil {
			t.Errorf("Expected nil. Got %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	name := "mymap"
	key, value := "my-key", "my-value"
	err = c.NewDMap(name).Put(key, value)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	err = c.NewDMap(name).Delete(key)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	_, err = c.NewDMap(name).Get(key)
	if err != olric.ErrKeyNotFound {
		t.Fatalf("Expected ErrKeyNotFound. Got: %v", err)
	}
}

func TestClient_LockWithTimeout(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		serr := db.Shutdown(context.Background())
		if serr != nil {
			t.Errorf("Expected nil. Got %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	name := "mymap"
	key := "my-key"
	err = c.NewDMap(name).Put(key, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	err = c.NewDMap(name).LockWithTimeout(key, time.Second)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	dm := db.NewDMap(name)
	err = dm.Unlock(key)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}
}

func TestClient_Unlock(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		serr := db.Shutdown(context.Background())
		if serr != nil {
			t.Errorf("Expected nil. Got %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	name := "mymap"
	key := "my-key"
	err = c.NewDMap(name).Put(key, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	err = c.NewDMap(name).LockWithTimeout(key, time.Second)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	err = c.NewDMap(name).Unlock(key)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}
}

func TestClient_Destroy(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		serr := db.Shutdown(context.Background())
		if serr != nil {
			t.Errorf("Expected nil. Got %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	name := "mymap"
	key := "my-key"
	err = c.NewDMap(name).Put(key, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	err = c.NewDMap(name).Destroy()
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	_, err = c.NewDMap(name).Get(key)
	if err != olric.ErrKeyNotFound {
		t.Fatalf("Expected ErrKeyNotFound. Got: %v", err)
	}
}

func TestClient_Incr(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		serr := db.Shutdown(ctx)
		if serr != nil {
			log.Printf("[WARN] Olric Shutdown returned an error: %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	key := "incr"
	name := "atomic_test"
	var wg sync.WaitGroup
	start := make(chan struct{})

	incr := func() {
		defer wg.Done()
		<-start

		_, ierr := c.NewDMap(name).Incr(key, 1)
		if ierr != nil {
			log.Printf("[ERROR] Failed to call Incr: %v", ierr)
			return
		}
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go incr()
	}

	close(start)
	wg.Wait()

	res, err := c.NewDMap(name).Incr(key, 1)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	if res != 101 {
		t.Fatalf("Expected 101. Got: %v", res)
	}

}

func TestClient_Decr(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		serr := db.Shutdown(ctx)
		if serr != nil {
			log.Printf("[WARN] Olric Shutdown returned an error: %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	key := "incr"
	name := "atomic_test"
	var wg sync.WaitGroup
	start := make(chan struct{})

	incr := func() {
		defer wg.Done()
		<-start

		_, ierr := c.NewDMap(name).Decr(key, 1)
		if ierr != nil {
			log.Printf("[ERROR] Failed to call Decr: %v", ierr)
			return
		}
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go incr()
	}

	close(start)
	wg.Wait()

	res, err := c.NewDMap(name).Decr(key, 1)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	if res != -101 {
		t.Fatalf("Expected -101. Got: %v", res)
	}

}

func TestClient_GetPut(t *testing.T) {
	db, done, err := newOlric()
	if err != nil {
		t.Fatalf("Expected nil. Got %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		serr := db.Shutdown(ctx)
		if serr != nil {
			log.Printf("[WARN] Olric Shutdown returned an error: %v", serr)
		}
		<-done
	}()

	c, err := New(testConfig, nil)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	key := "getput"
	name := "atomic_test"
	var total int64
	var wg sync.WaitGroup
	var final int64
	start := make(chan struct{})

	getput := func(i int) {
		defer wg.Done()
		<-start

		oldval, ierr := c.NewDMap(name).GetPut(key, i)
		if ierr != nil {
			log.Printf("[ERROR] Failed to call GetPut: %v", ierr)
			return
		}
		if oldval != nil {
			atomic.AddInt64(&total, int64(oldval.(int)))
		}
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go getput(i)
		final += int64(i)
	}

	close(start)
	wg.Wait()

	last, err := c.NewDMap(name).Get(key)
	if err != nil {
		t.Fatalf("Expected nil. Got: %v", err)
	}

	atomic.AddInt64(&total, int64(last.(int)))
	if atomic.LoadInt64(&total) != final {
		t.Fatalf("Expected %d. Got: %d", final, atomic.LoadInt64(&total))
	}
}
