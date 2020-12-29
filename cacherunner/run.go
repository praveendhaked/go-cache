package main

import (
	"gocache"
	"fmt"
	"time"
)

type readData struct {
	data gocache.Data
	err error
}

func showData(data gocache.Data, err error) {
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(data.GetKey()), ":", string(data.GetValue()), "read : ", data.GetReads(), "updates : ", data.GetUpdates())
}

// This is simple cost function for a entry where cost = length of key + length of value + number of reads - number of updates
var costFun = func(d gocache.Data) int {
	return len(d.GetKey())+len(d.GetValue())+d.GetReads()-d.GetUpdates()
}

func getData(k string, c *gocache.Cache) (gocache.Data, error) {
	return c.Get([]byte(k))
}

func addData(k, v string, c *gocache.Cache)  {
	err := c.Add([]byte(k), []byte(v), &costFun)
	if err!=nil {
		fmt.Println(err)
	}
}

func updateData(k, v string, c *gocache.Cache)  {
	err := c.Update([]byte(k), []byte(v))
	if err != nil {
		fmt.Println(err)
	}
}

func evictData(k string, c *gocache.Cache)  {
	err := c.Evict([]byte(k))
	if err != nil {
		fmt.Println(err)
	}
}

// Function for simulating multiple read operations
func readFromTo(start, end int, c *gocache.Cache, dataChannel chan readData)  {
	for i:=start ; i<end ; i++ {
		tmpK := fmt.Sprintf("key%v",i)
		x, y := getData(tmpK, c)
		dataChannel <- readData{x, y}
	}
	close(dataChannel)
}

// Function for simulating multiple add operations
func addFromTo(start, end int, c *gocache.Cache, ch chan bool)  {
	for i:= start ; i< end ; i++{
		tmpK, tmpV := fmt.Sprintf("key%v",i), fmt.Sprintf("val%v",i)
		addData(tmpK, tmpV, c)
		ch <- true
	}
}

func min(x, y int) int {
	if x<y {
		return x
	}
	return y
}

func main() {
	fmt.Println("\n***Simulation/Test-cases of Add, Get, Update and Evict methods***")

	var c gocache.Cache
	c.Init(3,1 )

	fmt.Println("\nInitialized cache with maximum number of entries = 3 and number of buckets = 1\n***(For demonstration of cost based eviction, I have taken capacity=3)***")

	fmt.Println("\nInserting <key1, value1>")
	addData("key1","value1", &c)
	fmt.Println("\nReading <key1>")
	result,err := getData("key1", &c)
	showData(result, err)
	if err!=nil {
		fmt.Println("\nTest Case failed")
	} else {
		if string(result.GetKey())=="key1" && string(result.GetValue()) == "value1" && result.GetReads()==1 && result.GetUpdates()==0 {
			fmt.Println("\nTest Case Passed")
		}else{
			fmt.Println("\nTest Case failed")
		}
	}

	fmt.Println("\nUpdating <key1, newValue>")
	updateData("key1", "newValue", &c)

	fmt.Println("\nReading <key1>")
	result,err = getData("key1", &c)
	showData(result, err)
	if err!=nil {
		fmt.Println("\nTest Case failed")
	} else {
		if string(result.GetKey())=="key1" && string(result.GetValue()) == "newValue" && result.GetReads()==2 && result.GetUpdates()==1 {
			fmt.Println("\nTest Case Passed")
		}else{
			fmt.Println("\nTest Case failed")
		}
	}

	fmt.Println("\nDeleting <key1>")
	evictData("key1", &c)

	fmt.Println("\nReading <key1>")
	result,err = getData("key1", &c)
	showData(result, err)

	if err!=nil {
		fmt.Println("\nTest Case Passed")
	} else {
		fmt.Println("\nTest Case Failed")
	}

	fmt.Println("\nAdding <key1, val1>, <key2, val22>, <key3, val333>, <key4, val4444>")

	addData("key1", "val1", &c)
	result,_ = getData("key1", &c)
	fmt.Println("\nCost of key1 = ", costFun(result))

	addData("key2", "val22", &c)
	result,_ = getData("key2", &c)
	fmt.Println("\nCost of key2 = ", costFun(result))

	addData("key3", "val333", &c)
	result,_ = getData("key3", &c)
	fmt.Println("\nCost of key3 = ", costFun(result))

	addData("key4", "val4444", &c)
	result,_ = getData("key4", &c)
	fmt.Println("\nCost of key4 = ", costFun(result))

	fmt.Println("\nWe have cache capacity = 3. Minimum cost key should be evicted now.")

	fmt.Println("\nReading <key1>")
	result,err = getData("key1", &c)
	showData(result, err)
	if err!=nil {
		fmt.Println("\nTest Case Passed")
	} else {
		fmt.Println("\nTest Case Failed")
	}

	fmt.Println("\nReading <key2>")
	result,err = getData("key2", &c)
	showData(result, err)

	if err==nil {
		fmt.Println("\nTest Case Passed")
	} else {
		fmt.Println("\nTest Case Failed")
	}

	fmt.Println("\nReading <key3>")
	result,err = getData("key3", &c)
	showData(result, err)

	if err==nil {
		fmt.Println("\nTest Case Passed")
	} else {
		fmt.Println("\nTest Case Failed")
	}

	fmt.Println("\nReading <key4>")
	result,err = getData("key4", &c)
	showData(result, err)

	if err==nil {
		fmt.Println("\nTest Case Passed")
	} else {
		fmt.Println("\nTest Case Failed")
	}

	fmt.Println("\nUpdating <key2, 13-char value>")
	updateData("key2", "13-char value", &c)

	result,_ = getData("key2", &c)
	fmt.Println("\nCost of key2 = ", costFun(result))
	result,_ = getData("key3", &c)
	fmt.Println("\nCost of key3 = ", costFun(result))
	result,_ = getData("key4", &c)
	fmt.Println("\nCost of key4 = ", costFun(result))

	fmt.Println("\nAdding <key5, value5>")
	fmt.Println("\nWe have cache capacity = 3. Minimum cost key should be evicted now.")
	addData("key5", "val5", &c)

	fmt.Println("\nReading <key3>")
	result,err = getData("key3", &c)
	showData(result, err)
	if err!=nil {
		fmt.Println("\nTest Case Passed")
	} else {
		fmt.Println("\nTest Case Failed")
	}
	c.Clear()

	fmt.Println("\n***Simulation to demonstrate the effect of concurrent access on performance***")
	fmt.Println("\nCapacity of cache is set to 1000000 entries and number of buckets is default = 512")

	c.Init(1000000, 0)

	n := 1000000

	fmt.Println("\nGenerating",n,"<k,v> pairs as <key1, val1>, <key1, val1>, ..., <key1000000, val1000000>")
	fmt.Println("\nAdding to cache sequentially.")
	tstart := time.Now()
	for i:=0 ; i<n ; i++ {
		tmpK, tmpV := fmt.Sprintf("key%v",i), fmt.Sprintf("val%v",i)
		addData(tmpK, tmpV, &c)
	}
	tend := time.Now()
	fmt.Println("Time taken in", n, "sequential Add calls", tend.Sub(tstart))

	fmt.Println("Total number of entries in cache after sequential Add calls", c.GetEntriesCount())

	fmt.Println("\nReading from cache sequentially.")
	tstart = time.Now()
	for i:=0 ; i<n ; i++ {
		tmpK:= fmt.Sprintf("key%v",i)
		_, _= getData(tmpK, &c)
	}
	tend = time.Now()
	fmt.Println("Time taken in", n, "sequential reads", tend.Sub(tstart))

	c.Clear()

	fmt.Println("\n***Total number of entries in cache after clearing the cache", c.GetEntriesCount(),"***")

	fmt.Println("\nAdding to cache concurrently using 100 go routines.")
	tstart = time.Now()
	for i:=0 ; i*10000<n ; i++ {
		go func(num int) {
			ch := make(chan bool, 10000)
			addFromTo(num*10000, min(n, (num+1)*10000), &c, ch)
			for _ = range ch{
				// This loop is just for getting values out of buffered channel
			}
		}(i)
	}
	tend = time.Now()
	fmt.Println("time taken in", n, "concurrent Add calls", tend.Sub(tstart))

	time.Sleep(2*time.Second)
	fmt.Println("Total number of entries in cache after concurrent Add calls", c.GetEntriesCount())

	fmt.Println("\nReading from cache concurrently using 100 go routines.")
	tstart = time.Now()
	for i:=0 ; i*10000<n ; i++ {
		go func(num int) {
			ch := make(chan readData, 10000)
			readFromTo(num*10000, min(n, (num+1)*10000), &c, ch)
			for _ = range ch{
				// This loop is just for getting values out of buffered channel
			}
		}(i)
	}

	tend = time.Now()
	fmt.Println("time taken in", n, "concurrent reads", tend.Sub(tstart))

	fmt.Println("\n***Here It can be seen that total number of entries in cache are little less than the added entries,\n" +
		"while we have specified the capacity of the cache already. It is happening because the specified capacity was divided into the buckets of equal size.\n" +
		"The bucket is chosen according to the hash value of the key. Some buckets may get more keys than others and which may lead to eviction of keys from them when those buckets are full.\n" +
		"That's why total number of entries is little less than the added entries. I have used 64-bit hash function from hash/fnv library.***")
}

