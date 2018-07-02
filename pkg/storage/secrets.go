package storage

import (
	"github.com/solo-io/gloo/pkg/storage/dependencies"
	"github.com/solo-io/gloo/pkg/storage"
	"github.com/pkg/errors"
	"sort"
	"github.com/solo-io/gloo/pkg/log"
	"sync"
)

/**************************************
	Base
 *************************************/

type InMemorySecrets struct {
	lock    sync.RWMutex
	objects map[string]*dependencies.Secret
	updates chan struct{}
}

func NewInMemorySecrets() *InMemorySecrets {
	return &InMemorySecrets{
		objects: make(map[string]*dependencies.Secret),
		updates: make(chan struct{}, 10),
	}
}

var _ dependencies.SecretStorage = &InMemorySecrets{}

func (s *InMemorySecrets) updated() {
	go func() {
		s.updates <- struct{}{}
	}()
}

func (s *InMemorySecrets) Create(obj *dependencies.Secret) (*dependencies.Secret, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, exists := s.objects[obj.Ref]; exists {
		return nil, storage.NewAlreadyExistsErr(errors.Errorf("%s already exists", obj.Ref))
	}
	s.objects[obj.Ref] = obj
	s.updated()
	return obj, nil
}

func (s *InMemorySecrets) Update(obj *dependencies.Secret) (*dependencies.Secret, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, exists := s.objects[obj.Ref]; !exists {
		return nil, errors.Errorf("%s does not exist", obj.Ref)
	}
	s.objects[obj.Ref] = obj
	s.updated()
	return obj, nil
}

func (s *InMemorySecrets) Delete(name string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, exists := s.objects[name]; !exists {
		return errors.Errorf("%s does not exist", name)
	}
	delete(s.objects, name)
	s.updated()
	return nil
}

func (s *InMemorySecrets) Get(name string) (*dependencies.Secret, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	obj, exists := s.objects[name]
	if !exists {
		return nil, errors.Errorf("%s does not exist", name)
	}
	return obj, nil
}

func (s *InMemorySecrets) List() ([]*dependencies.Secret, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var objs []*dependencies.Secret
	for _, obj := range s.objects {
		objs = append(objs, obj)
	}
	sort.SliceStable(objs, func(i, j int) bool {
		return objs[i].Ref < objs[j].Ref
	})
	return objs, nil
}

func (s *InMemorySecrets) Watch(handlers ... dependencies.SecretEventHandler) (*storage.Watcher, error) {
	return storage.NewWatcher(func(stop <-chan struct{}, errs chan error) {
		for {
			select {
			case <-s.updates:
				secrets, err := s.List()
				if err != nil {
					log.Warnf("failed to list config objects: %v", err)
					continue
				}
				for _, h := range handlers {
					h.OnUpdate(secrets, nil)
				}
			case err := <-errs:
				log.Warnf("failed to start watcher: %v", err)
				return
			case <-stop:
				return
			}
		}
	}), nil
}
