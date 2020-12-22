// Copyright 2020 ETH Zurich
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package queue

import "sync"

type Sortable interface {
	Less(b Sortable) bool
}

type Queue interface {
	Pop() Sortable
	Peek() Sortable
	Push(sortable Sortable)
	Len() int
}

var _ Queue = &Implementation{}

// This implementation is not intended to be used directly.
// Rather appropriate wrapper method should be created
// that prevent the wrong types from being added
type Implementation struct {
	first *Item
	len   int
	sync.Mutex
}

type Item struct {
	value      Sortable
	prev, next *Item
}

// Time-Complexity: O(1)
func (queue *Implementation) Pop() Sortable {
	queue.Lock()
	defer queue.Unlock()

	if queue.first == nil {
		panic("Can't pop from empty queue")
	}

	queue.len--

	first := queue.first.value
	if queue.first.next != nil {
		queue.first = queue.first.next
		queue.first.prev = nil
	} else {
		queue.first = nil
	}

	return first
}

// Time-Complexity: O(n)
func (queue *Implementation) Push(sortable Sortable) {
	queue.Lock()
	defer queue.Unlock()

	queue.len++

	item := &Item{value: sortable}
	if queue.first == nil {
		queue.first = item
	} else if sortable.Less(queue.first.value) {
		item.next = queue.first
		queue.first.prev = item
		queue.first = item
	} else {
		cur := queue.first
		for cur.next != nil && cur.next.value.Less(sortable) {
			cur = cur.next
		}

		if cur.next != nil {
			item.next = cur.next
			item.next.prev = item
		}

		cur.next = item
		item.prev = cur
	}
}

func (queue *Implementation) Peek() Sortable {
	queue.Lock()
	defer queue.Unlock()

	return queue.first.value
}

// Time-Complexity: O(1)
func (queue *Implementation) Len() int {
	queue.Lock()
	defer queue.Unlock()

	return queue.len
}

func NewQueue() Queue {
	return &Implementation{}
}
