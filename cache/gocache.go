package gocache

import (
	"bytes"
	"errors"
	"hash/fnv"
	"math"
	"sync"
	"sync/atomic"
)

const defaultBucketsNumber = 512

const maxEntriesPerBucket = 2000

type Data struct {
	key          []byte
	value        []byte
	reads        int
	updates      int
	costFunction *func(data Data) int	//pointer to cost function associated with this entry
	next         *Data
	prev         *Data
}

func (data Data) GetKey() []byte {
	return data.key
}

func (data Data) GetValue() []byte {
	return data.value
}

func (data Data) GetReads() int {
	return data.reads
}
func (data Data) GetUpdates() int {
	return data.updates
}

type bucket struct {
	mutex        sync.RWMutex
	entries      map[uint64]*Data		// key of this map = hash(key) and value of this map is pointer to Data
	costListsMap map[int]*dataNodesList	// key of this map is cost, value of this map is doubly linked list of Data nodes with the same cost
	costTree     *costNode				// root node of AVL Tree, tree stores costNodes
	maxEntries   uint64					// maximum number of entries in the bucket
	entriesCount uint64					// current number of entries in the bucket
	collisions   uint64					// count of collisions due to same hash of different keys in the bucket
}

type Cache struct {
	buckets []bucket
}

//Doubly linked list
type dataNodesList struct {
	size int
	head *Data
	tail *Data
}

// Create a new doubly linked list
func createDataNodesList() *dataNodesList {
	return &dataNodesList{0, nil, nil}
}

// Add nodes to linked list, it will add node at the tail of linked list
func (list *dataNodesList) addNode(node *Data)  {
	node.next=nil
	node.prev=nil

	if list.head==nil && list.tail==nil{
		list.head = node
	}else{
		list.tail.next = node
		node.prev = list.tail
	}

	list.tail = node
	list.size++
}

// Remove node from linked list
func (list *dataNodesList) removeNode(node *Data)  {
	if node==list.head {
		list.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}
	if node.prev != nil {
		node.prev.next = node.next
	}
	if list.tail == node {
		list.tail = node.prev
	}

	node.next=nil
	node.prev=nil
	list.size--
}

// Retruns sum of collisions count over all the buckets in cache
func (c *Cache) GetCollisionsCount() uint64 {
	var sum uint64 = 0
	for i:=0 ; i<len(c.buckets) ; i++ {
		sum += atomic.LoadUint64(&c.buckets[i].collisions)
	}
	return sum
}

// Init method for cache
func (c *Cache) Init(capacity int, buckets int) {
	if buckets < 0 {
		panic("Number of buckets can not be negative. You can use 0 for default number of buckets = 512")
	}
	if buckets > 1024 {
		panic("Maximum number of buckets restricted to 1024")
	}

	var numberOfBuckets int
	if buckets==0{
		numberOfBuckets = defaultBucketsNumber
	} else {
		numberOfBuckets = buckets
	}

	if capacity < 0 {
		panic("Capacity must be a positive int")
	}
	if capacity > numberOfBuckets*maxEntriesPerBucket {
		panic("Capacity should be less than number of buckets times 2000")
	}

	c.buckets = make([]bucket, numberOfBuckets)

	for i:=0 ; i<numberOfBuckets ; i++ {
		c.buckets[i].initBucket(min(maxEntriesPerBucket, int(math.Ceil(float64(capacity)/float64(numberOfBuckets)))))
	}
}

// Clear method for cache
func (c *Cache) Clear() {
	for i:=0 ; i<len(c.buckets) ; i++ {
		c.buckets[i].clearBucket()
	}
}

func (b *bucket) initBucket(bucketCapacity int) {
	b.mutex.Lock()
	b.entries = map[uint64]*Data{}
	b.costListsMap = map[int]*dataNodesList{}
	atomic.StoreUint64(&b.maxEntries, uint64(bucketCapacity))
	atomic.StoreUint64(&b.entriesCount, 0)
	atomic.StoreUint64(&b.collisions, 0)
	b.mutex.Unlock()
}

func (b *bucket) clearBucket()  {
	b.mutex.Lock()
	b.entries = map[uint64]*Data{}
	b.costTree = nil
	b.costListsMap = map[int]*dataNodesList{}
	atomic.StoreUint64(&b.entriesCount, 0)
	atomic.StoreUint64(&b.collisions, 0)
	b.mutex.Unlock()
}

// Returns sum of total entries count in the cache
func (c *Cache) GetEntriesCount() uint64 {
	var count uint64
	for i:=0 ; i<len(c.buckets) ; i++ {
		c.buckets[i].mutex.RLock()
		count = count + atomic.LoadUint64(&c.buckets[i].entriesCount)
		c.buckets[i].mutex.RUnlock()
	}
	return count
}

// Add method will add (k, v) to the cache
func (c *Cache) Add(k, v []byte, costFun *func(data Data) int) error {
	h := getHash64(k)
	return c.buckets[h%uint64(len(c.buckets))].addToBucket(k, v, h, costFun)
}

// Get method will return the (k, v) for matched k
func (c *Cache) Get(k []byte) (Data, error) {
	if c==nil {
		return Data{}, errors.New("Cache has not been initialized. Use Init() method for initialization.")
	}
	h := getHash64(k)
	return c.buckets[h%uint64(len(c.buckets))].getFromBucket(k, h)
}

// Update method will update the v for given k
func (c *Cache) Update(k, v []byte) error {
	if c==nil {
		return errors.New("Cache has not been initialized. Use Init() method for initialization.")
	}
	h := getHash64(k)
	return c.buckets[h%uint64(len(c.buckets))].updateInBucket(k, v, h)
}

// Evict method will evict the (k, v) from the cache on the basis of k
func (c *Cache) Evict(k []byte) error {
	if c==nil {
		return errors.New("Cache has not been initialized. Use Init() method for initialization.")
	}
	h := getHash64(k)
	return c.buckets[h%uint64(len(c.buckets))].deleteFromBucket(k, h)
}

func (b *bucket) addToBucket(k, v []byte, h uint64, costFun *func(data Data) int) error {
	if b.entries == nil {
		return errors.New("Cache has not been initialized. Use Init() method for initialization.")
	}
	b.mutex.Lock()
	node := &Data{k, v, 0, 0, costFun, nil, nil}

	value, exist := b.entries[h]

	if exist {
		if !bytes.Equal(value.key, k) {
			atomic.AddUint64(&b.collisions, 1)
		}
		delete(b.entries, h)
		cost := (*value.costFunction)(*value)
		b.costListsMap[cost].removeNode(value)
		if b.costListsMap[cost].size == 0 {
			delete(b.costListsMap, cost)
			b.costTree = remove(b.costTree, cost)
		}
		b.entriesCount = uint64(len(b.entries))
	}

	if b.entriesCount == b.maxEntries {
		minCostNode := findMinimum(b.costTree)
		if minCostNode != nil {
			minCost := minCostNode.cost

			headNode := b.costListsMap[minCost].head
			delete(b.entries, getHash64(headNode.key))

			b.costListsMap[minCost].removeNode(b.costListsMap[minCost].head)
			if b.costListsMap[minCost].size == 0 {
				delete(b.costListsMap, minCost)
				b.costTree = remove(b.costTree, minCost)
			}
			b.entriesCount = uint64(len(b.entries))
		}
	}
	b.entries[h] = node
	nodeCost := (*node.costFunction)(*node)
	b.costTree = insert(b.costTree, nodeCost)
	_, found := b.costListsMap[nodeCost]
	if found {
		b.costListsMap[nodeCost].addNode(node)
	}else{
		b.costListsMap[nodeCost] = createDataNodesList()
		b.costListsMap[nodeCost].addNode(node)
	}
	b.entriesCount = uint64(len(b.entries))

	b.mutex.Unlock()
	return nil
}

func (b *bucket) getFromBucket(k []byte, h uint64) (Data, error) {
	if b.entries == nil {
		return Data{}, errors.New("Cache has not been initialized. Use Init() method for initialization.")
	}
	b.mutex.Lock()
	value, exist := b.entries[h]

	if !exist {
		b.mutex.Unlock()
		return Data{}, errors.New("key-value pair not found")
	} else {
		if bytes.Compare(k, value.key) != 0 {
			atomic.AddUint64(&b.collisions, 1)
			b.mutex.Unlock()
			return Data{}, errors.New("key-value pair not found")
		}
	}

	oldCost := (*value.costFunction)(*value)
	value.reads++
	newCost := (*value.costFunction)(*value)

	if oldCost != newCost{

		nodesList := b.costListsMap[oldCost]
		if nodesList.size == 1 {
			b.costTree = remove(b.costTree, oldCost)
			delete(b.costListsMap, oldCost)
		}else {
			nodesList.removeNode(value)
		}

		_, found := b.costListsMap[newCost]

		if found{
			b.costListsMap[newCost].addNode(value)
		}else {
			b.costListsMap[newCost] = createDataNodesList()
			b.costListsMap[newCost].addNode(value)

			b.costTree = insert(b.costTree, newCost)
		}
	}

	b.mutex.Unlock()

	return *value, nil
}

func (b *bucket) updateInBucket(k, v []byte, h uint64) error {
	if b.entries == nil {
		return errors.New("Cache has not been initialized. Use Init() method for initialization.")
	}
	b.mutex.Lock()
	value, exist := b.entries[h]
	if exist {
		if bytes.Compare(k, value.key) == 0 {
			oldCost := (*value.costFunction)(*value)
			value.value = v
			value.updates++
			newCost := (*value.costFunction)(*value)
			if oldCost!=newCost{
				nodesList := b.costListsMap[oldCost]
				if nodesList.size==1 {
					b.costTree = remove(b.costTree, oldCost)
					delete(b.costListsMap, oldCost)
				}else {
					nodesList.removeNode(value)
				}

				_, found := b.costListsMap[newCost]

				if found{
					b.costListsMap[newCost].addNode(value)
				}else {
					b.costListsMap[newCost] = createDataNodesList()
					b.costListsMap[newCost].addNode(value)

					b.costTree = insert(b.costTree, newCost)
				}
			}
			b.mutex.Unlock()
			return nil
		}
		atomic.AddUint64(&b.collisions, 1)
	}
	b.mutex.Unlock()
	return errors.New("key not exist")
}

func (b *bucket) deleteFromBucket(k []byte, h uint64) error {
	if b.entries == nil {
		return errors.New("Cache has not been initialized. Use Init() method for initialization.")
	}
	b.mutex.Lock()
	value, exist := b.entries[h]
	if exist {
		if bytes.Compare(k, value.key) == 0 {
			cost := (*value.costFunction)(*value)
			nodesList := b.costListsMap[cost]
			if nodesList.size==1{
				b.costTree = remove(b.costTree, cost)
				delete(b.costListsMap, cost)
			}else{
				nodesList.removeNode(value)
			}
			delete(b.entries, h)
			b.entriesCount = uint64(len(b.entries))
			b.mutex.Unlock()
			return nil
		}
		atomic.AddUint64(&b.collisions, 1)
	}
	b.mutex.Unlock()
	return errors.New("key not exist")
}

func getHash64(k []byte) uint64 {
	hasher := fnv.New64a()
	_, err := hasher.Write(k)
	if err != nil {
		return 0
	}
	return hasher.Sum64()
}

type costNode struct {
	cost int
	height int
	left *costNode
	right *costNode
}

// returns a new Node for the tree
func newCostNode(cost int) *costNode {
	return &costNode{cost, 1, nil, nil}
}

func height(node *costNode) int {
	if node == nil {
		return 0
	}
	return node.height
}

func max(a, b int) int {
	if a>b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a<b {
		return a
	}
	return b
}

// Right rotation in tree
func rightRotate(node *costNode) *costNode {
	leftNode := node.left
	leftRightSubTree := leftNode.right

	leftNode.right = node
	node.left = leftRightSubTree

	node.height = max(height(node.left), height(node.right))+1
	leftNode.height = max(height(leftNode.left), height(leftNode.right))+1

	return leftNode
}

// Left rotation in tree
func leftRotate(node *costNode) *costNode {
	rightNode := node.right
	rightLeftSubTree := rightNode.left

	rightNode.left = node
	node.left = rightLeftSubTree

	node.height = max(height(node.left), height(node.right))+1
	rightNode.height = max(height(rightNode.left), height(rightNode.right))+1

	return rightNode
}

func getHeightDiff(node *costNode) int {
	if node==nil {
		return 0
	}

	return height(node.left)-height(node.left)
}

// Insert into cost tree
func insert(rootNode *costNode, cost int) *costNode {
	if rootNode ==nil {
		return newCostNode(cost)
	}

	if cost < rootNode.cost {
		rootNode.left = insert(rootNode.left, cost)
	} else if cost > rootNode.cost {
		rootNode.right = insert(rootNode.right, cost)
	} else {
		return rootNode
	}

	rootNode.height = 1+max(height(rootNode.left), height(rootNode.right))

	heightDiff := getHeightDiff(rootNode)

	if heightDiff>1 {
		if cost < rootNode.left.cost {
			return rightRotate(rootNode)
		}

		if cost > rootNode.left.cost {
			rootNode.left = leftRotate(rootNode.left)
			return rightRotate(rootNode)
		}
	}

	if heightDiff < -1 {
		if cost > rootNode.right.cost {
			return leftRotate(rootNode)
		}
		if cost < rootNode.right.cost {
			rootNode.right = rightRotate(rootNode.right)
			return leftRotate(rootNode)
		}
	}
	return rootNode
}

// remove from cost tree
func remove(rootNode *costNode, cost int) *costNode {
	if rootNode ==nil {
		return rootNode
	}

	if cost < rootNode.cost {
		rootNode.left = remove(rootNode.left, cost)
	} else if cost > rootNode.cost {
		rootNode.right = remove(rootNode.right, cost)
	} else {
		if rootNode.left==nil || rootNode.right==nil {
			var ptr *costNode
			if rootNode.left==nil {
				ptr = rootNode.right
			} else {
				ptr = rootNode.left
			}

			if ptr==nil {
				rootNode = nil
			}else{
				rootNode = ptr
			}
		}else{
			var ptr = rootNode.right
			for ptr.left != nil {
				ptr = ptr.left
			}
			rootNode.cost = ptr.cost
			rootNode.right = remove(rootNode.right, ptr.cost)
		}
	}

	if rootNode ==nil {
		return rootNode
	}

	rootNode.height = 1+max(height(rootNode.left), height(rootNode.right))

	heightDiff := getHeightDiff(rootNode)

	if heightDiff > 1 {
		if getHeightDiff(rootNode.left) >=0 {
			return rightRotate(rootNode)
		}
		if getHeightDiff(rootNode.left) <0 {
			rootNode.left = leftRotate(rootNode.left)
			return rightRotate(rootNode)
		}
	}

	if heightDiff < -1 {
		if getHeightDiff(rootNode.right) <= 0 {
			return leftRotate(rootNode)
		}

		if getHeightDiff(rootNode.right) > 0 {
			rootNode.right = rightRotate(rootNode.right)
			return leftRotate(rootNode)
		}
	}

	return rootNode
}

// Returns minimum cost node of the cost tree
func findMinimum(rootNode *costNode) *costNode {
	node := rootNode
	for node!=nil && node.left != nil {
		node = node.left
	}
	return node
}