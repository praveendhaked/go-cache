# Go-Cache

A caching library written in `Golang`. This library uses cost based eviction strategy, where cost function for each entry will be provided by library user.

### How to use the library ?
To use the cache library, one needs to download it and import in the project. See, below code snippet for using the library:-

```
package main

import (
    "megacache"
    "fmt"
)

func main()  {
    var cache megacache.Cache
	
    // Initialize the cache
    // First input field is capacity = maximum number of entries
    // Second input field is buckets = number of buckets (buckets are described below)
    cache.Init(3, 1)

    // Adding <key, value> in cache
    err := cache.Add([]byte("key"), []byte("value"), &costFunction)
    
    if err != nil {
        fmt.Println(err)
    }
}

// A cost function which return cost = length of key + length of value
var costFunction = func(data megacache.Data) (int) {
	return len(data.GetKey())+len(data.GetValue())
}
```

### cachemodule

This module contains cache library.

#### Megacache library has following functions:-

1. _Add(key, value, *costFunction)_ : This function will add <key, value> pair in the cache along with a cost function pointer for this entry. If the cache will be full then it will evict minimum Cost entry from the cache before adding the entry. In the input of this function key, value are slice of bytes and costFunction is the pointer to user defined cost function. It returns _error_ if there is any otherwise nil.

2. _Get(key)_ : This function will be used to read a key from cache. In the input of this function key is a slice of bytes. It returns two values of type _megacache.Data_ and _error_. error will be nil if there is not any error.

3. _Evict(key)_ : This function will be used to remove a key from cache. In the input of this function key is a slice of bytes. It returns _error_. error will be nil if there is not any error.

4. _Update(key, value)_ : This function will update the value for a given key if the key exist in the cache. In the input of this function key, value are slice of bytes. It returns _error_. error will be nil if there is not any error.


### Inside the MegaCache Library

#### Concurrency
As the library may receive many requests concurrently. So, we need to provide concurrent access to our cache. To enable the concurrent access to cache we can use `sync.RWMutex` in the methods of our cache. But, It will block the whole cache when a goroutine will be modifying the cache and cache performance will be very poor.

To overcome this issue we have divided our cache in multiple `buckets`. Now, each go routine will use the `sync.RWMutex` only on the bucket it is modifying. This way whole cache will not be locked and concurrently multiple go routines will be able to access the cache faster. However, one bucket can only be modified by one go routine at a time.

The bucket of a `key` is decided by generating 64-bit hash of the key and taking `modulo` with maximum number of buckets in the cache. For generating 64-bit hash, `hash/fnv` library has been used.

### Cost based eviction
We are using a user defined cost function for calculating the cost of each entry. User need to provide cost function at the time of adding entry to cache, the cost of the that key will be calculated using that cost function only. Cost function has the signature _func(data *megacache.Data)  (int)_.
The cost of an entry can change at time of _update_ or _get_ operations also. So we need to re-balance costs after each operation. Also, For cost based eviction from cache we need to get the entry with minimum cost for evicting.

One way was to iterate over all entries to find the minimum with `O(N)` complexity and other way can be to use `minHeap`.
`minHeap` makes it possible to get minimum in `O(1)` but we need to delete that minimum entry also. So, it becomes `O(log(N))`. Also, Our requirement is to update the cost of any entry, in the `heap`, whenever it changes.

Finally, it boils down to implement such a data structure which can do `insert`, `delete` and `getMinimum` operations efficiently.
To achieving these operations efficiently, I have implemented `self-balancing binary search tree` using `AVL Tree`. So it made all these operations in `O(log(N))`.


### cacherunner

This module has testcases and simulations.

#### Results after running TestCase/Simulations written in run.go of cacherunner module
```
***Simulation/Test-cases of Add, Get, Update and Evict methods***

Initialized cache with maximum number of entries = 3 and number of buckets = 1
***(For demonstration of cost based eviction, I have taken capacity=3)***

Inserting <key1, value1>

Reading <key1>
key1 : value1 read :  1 updates :  0

Test Case Passed

Updating <key1, newValue>

Reading <key1>
key1 : newValue read :  2 updates :  1

Test Case Passed

Deleting <key1>

Reading <key1>
key-value pair not found

Test Case Passed

Adding <key1, val1>, <key2, val22>, <key3, val333>, <key4, val4444>

Cost of key1 =  9

Cost of key2 =  10

Cost of key3 =  11

Cost of key4 =  12

We have cache capacity = 3. Minimum cost key should be evicted now.

Reading <key1>
key-value pair not found

Test Case Passed

Reading <key2>
key2 : val22 read :  2 updates :  0

Test Case Passed

Reading <key3>
key3 : val333 read :  2 updates :  0

Test Case Passed

Reading <key4>
key4 : val4444 read :  2 updates :  0

Test Case Passed

Updating <key2, 13-char value>

Cost of key2 =  19

Cost of key3 =  13

Cost of key4 =  14

Adding <key5, value5>

We have cache capacity = 3. Minimum cost key should be evicted now.

Reading <key3>
key-value pair not found

Test Case Passed

***Simulation to demonstrate the effect of concurrent access on performance***

Capacity of cache is set to 1000000 entries and number of buckets is default = 512

Generating 1000000 <k,v> pairs as <key1, val1>, <key1, val1>, ..., <key1000000, val1000000>

Adding to cache sequentially.
Time taken in 1000000 sequential Add calls 1.3655364s
Total number of entries in cache after sequential Add calls 996386

Reading from cache sequentially.
Time taken in 1000000 sequential reads 625.1398ms

***Total number of entries in cache after clearing the cache 0 ***

Adding to cache concurrently using 100 go routines.
time taken in 1000000 concurrent Add calls 1.0025ms
Total number of entries in cache after concurrent Add calls 996386

Reading from cache concurrently using 100 go routines.
time taken in 1000000 concurrent reads 997.8µs

***Here It can be seen that total number of entries in cache are little less than the added entries,
while we have specified the capacity of the cache already. It is happening because the specified capacity was divided into the buckets of equal size.
The bucket is chosen according to the hash value of the key. Some buckets may get more keys than others and which may lead to eviction of keys from them when those buckets are full.
That's why total number of entries is little less than the added entries. I have used 64-bit hash function from hash/fnv library.***
```

### Assumptions
1. If the `hash` of two keys is same then we will remove existing key from the cache and add the newer key. Just for keeping the library simple it was done so. :)
2. Inputs in the functions of cache library are kept as slice of bytes. It is has been assumed that user will serialize the data into slice of bytes before calling cache methods.
3. The cache library assigns equal capacity to each bucket. capacity of each bucket will be `ceil(capacity/numberOfBuckets)`. For example:- If we give capacity=4 and buckets=3 at the initialization of cache then it will create 3 buckets, each with a capacity of ceil(4/3) = 2.
