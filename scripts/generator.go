package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/samuel/go-zookeeper/zk"
)

const (
	ELEMENT_NUMBER = 10
	TIME_INTERVAL  = time.Second
)

// channel variables
var (
	done = make(chan bool)
	msgs = make(chan string)
)

// holds the flag variables
var (
	element    = flag.Int("element", 10, "default znode number")
	interval   = flag.Int("interval", 1, "default duration to generate strings")
	goroutines = flag.Int("goroutines", 2, "default go routines count")
)

var mu = &sync.Mutex{}

func main() {
	flag.Parse()

	if *goroutines > *element {
		fmt.Printf("%v", fmt.Errorf("err: goroutines should be less than element number"))
		return
	}

	c, _, err := zk.Connect([]string{"0.0.0.0"}, time.Second) //*10)
	if err != nil {
		log.Fatal(err.Error())
	}

	if err := cleanUp(c); err != nil {
		log.Fatal(err.Error())
	}

	childrenBefore, stat, _, err := c.ChildrenW("/")
	if err != nil {
		panic(err)
	}

	fmt.Printf("children is :%+v, stat is: %+v\n", childrenBefore, stat)

	sign := make(chan os.Signal)
	signal.Notify(sign, os.Interrupt, syscall.SIGTERM)

	closeChan := make(chan struct{}, 0)
	go handleCtrlC(sign, closeChan)

	go produce(time.Second*time.Duration(*interval), *element, closeChan)

	var wg sync.WaitGroup
	wg.Add(*goroutines)
	// multiple go routine to process created keys in zookeper data model
	for i := 0; i < *goroutines; i++ {
		go consume(c, &wg)
	}

	<-done
	wg.Wait()

	childrenAfter, stat, _, err := c.ChildrenW("/")
	if err != nil {
		panic(err)
	}

	if producer == consumer {
		fmt.Println("producer and consumer check is finihed as successfully")
	}

	fmt.Printf("children is :%+v, stat is: %+v\n", childrenAfter, stat)

	if len(childrenAfter) == *element+1 {
		fmt.Println("successfull")
	}
	os.Exit(0)
	// e := <-ch
	// fmt.Printf("%+v\n", e)
}

var (
	producer, consumer int
)

func produce(x time.Duration, n int, c chan struct{}) {
	defer func() {
		close(msgs)
		done <- true
	}()

	for i := 0; i < n; i++ {
		select {
		case s := <-c:
			fmt.Println("interrupt signal geldii!!!", s)
			return
		case <-time.After(x):
			str := fmt.Sprintf("/key-%d", i)
			msgs <- str
			producer++
		}
	}
}

func consume(c *zk.Conn, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		msg, ok := <-msgs
		if !ok {
			return
		}
		_, err := c.Create(msg, nil, 0, zk.WorldACL(zk.PermAll))
		if err != nil {
			if err != zk.ErrNodeExists {
				fmt.Println("ERROR IS :", err)
			}
		}
		if err == nil {
			mu.Lock()
			consumer++
			mu.Unlock()
		}

		fmt.Println(msg)
	}
}

// cleanUp cleans the all znodes in zookeper data structure
func cleanUp(c *zk.Conn) error {
	children, _, _, err := c.ChildrenW("/")
	if err != nil {
		return err
	}

	for _, child := range children {
		// delete operation takes path parameter
		// then it should have '/' as first index
		if err := c.Delete("/"+child, 0); err != nil {
			// we can't delete /zookeeper path from the znodes
			if child == "zookeeper" {
				continue
			}
			// fmt.Println("ERROR WHILE CLEANING", err)
			return err
		}
	}

	return nil
}

func handleCtrlC(c chan os.Signal, cc chan struct{}) {
	sig := <-c
	close(cc)
	// handle ctrl+c event here
	// for example, close database
	fmt.Println("\nsignal: ", sig)
	time.Sleep(2 * time.Second)
	os.Exit(0)
}
