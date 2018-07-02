package gloo

import (
	"github.com/solo-io/gloo/pkg/storage/dependencies"
	"github.com/solo-io/gloo/pkg/storage"
	"github.com/pkg/errors"
	"sort"
	"github.com/d4l3k/messagediff"
	"github.com/solo-io/gloo/pkg/log"
	"time"
	"sync"
)

type InMemorySecrets struct {
	lock sync.RWMutex
	updated  chan struct{}
	base dependencies.SecretStorage
	inMemory map[string]*dependencies.Secret
}

var _ dependencies.SecretStorage = &InMemorySecrets{}

func (sm *InMemorySecrets) signalUpdate() {
	go func() {
		sm.updated <- struct{}{}
	}()
}

func (sm *InMemorySecrets) Create(sec *dependencies.Secret) (*dependencies.Secret, error) {
	sm.lock.Lock()
	sm.inMemory[sec.Ref] = sec
	sm.lock.Unlock()
	sm.signalUpdate()
	return sec, nil
}

func (sm *InMemorySecrets) Update(sec *dependencies.Secret) (*dependencies.Secret, error) {
	return sm.Create(sec)
}

func (sm *InMemorySecrets) Delete(name string) error {
	sm.lock.Lock()
	delete(sm.inMemory, name)
	sm.lock.Unlock()
	sm.signalUpdate()
	return nil
}

func (sm *InMemorySecrets) Get(name string) (*dependencies.Secret, error) {
	sm.lock.RLock()
	sec, ok := sm.inMemory[name]
	sm.lock.RUnlock()
	if !ok {
		return sm.base.Get(name)
	}
	return sec, nil
}

func (sm *InMemorySecrets) List() ([]*dependencies.Secret, error) {
	baseList, err := sm.base.List()
	if err != nil {
		return nil, err
	}
	sm.lock.RLock()
	for _, sec := range sm.inMemory {
		baseList = append(baseList, sec)
	}
	sm.lock.RUnlock()
	return baseList, nil
}

func (sm *InMemorySecrets) Watch(handlers ... dependencies.SecretEventHandler) (*storage.Watcher, error) {
	baseWatch, err := sm.base.Watch(&dependencies.SecretEventHandlerFuncs{
		AddFunc: func(updatedList []*dependencies.Secret, obj *dependencies.Secret) {
			sm.signalUpdate()
		},
		UpdateFunc: func(updatedList []*dependencies.Secret, obj *dependencies.Secret) {
			sm.signalUpdate()
		},
		DeleteFunc: func(updatedList []*dependencies.Secret, obj *dependencies.Secret) {
			sm.signalUpdate()
		},
	})
	if err != nil {
		return nil, errors.Wrap(err, "creating base secret watcher")
	}
	var lastSeen []*dependencies.Secret
	update := func() error {
		list, err := sm.List()
		if err != nil {
			return errors.Wrap(err, "listing secrets")
		}
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].Ref < list[j].Ref
		})

		// no change since last poll
		if _, equal := messagediff.PrettyDiff(lastSeen, list); equal {
			return nil
		}

		// update index
		lastSeen = list
		for _, h := range handlers {
			h.OnUpdate(list, nil)
		}
		return nil
	}
	return storage.NewWatcher(func(stop <-chan struct{}, errs chan error) {
		go baseWatch.Run(stop, errs)
		for {
			select {
			case <-sm.updated:
				if err := update(); err != nil {
					log.Warnf("error syncing with secret backend: %v", err)
				}
			case err := <-errs:
				log.Warnf("failed to start secret watcher: %v", err)
				time.Sleep(time.Second)
				continue
			case <-stop:
				return
			}
		}
	}), nil
}
