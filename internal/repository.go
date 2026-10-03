package internal

import (
	"fmt"
	"sync"
)

type partrepository struct {
	mu      sync.Mutex
	storage map[int64]Part
	nextID  int
}

func NewPartRepository() *partrepository {
	return &partrepository{
		storage: make(map[int64]Part),
		nextID:  1,
	}
}

func (r *partrepository) GetAll() []Part {
	r.mu.Lock()
	defer r.mu.Unlock()

	var parts []Part
	for _, part := range r.storage {
		parts = append(parts, part)
	}
	return parts
}

func (r *partrepository) Create(part Part) Part {
	r.mu.Lock()
	defer r.mu.Unlock()

	part.ID = r.nextID
	r.storage[int64(r.nextID)] = part
	r.nextID++

	return part
}

func (r *partrepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, exists := r.storage[id]
	if !exists {
		fmt.Printf("Error: Part with ID %d not found\n", id)
	}
	delete(r.storage, id)

	return nil
}
