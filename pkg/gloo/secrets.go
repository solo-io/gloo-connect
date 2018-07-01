package gloo

import (
	"github.com/solo-io/gloo/pkg/storage/dependencies"
	"github.com/solo-io/gloo/pkg/storage"
	"github.com/pkg/errors"
	"sort"
	"github.com/d4l3k/messagediff"
	"github.com/solo-io/gloo/pkg/log"
	"time"
)

type SecretMerger struct {
	updated  chan struct{}
	dependencies.SecretStorage
	inMemory map[string]*dependencies.Secret
}

var _ dependencies.SecretStorage = &SecretMerger{}

func (sm *SecretMerger) signalUpdate() {
	go func() {
		sm.updated <- struct{}{}
	}()
}

func (sm *SecretMerger) Create(sec *dependencies.Secret) (*dependencies.Secret, error) {
	sm.inMemory[sec.Ref] = sec
	return sec, nil
}

func (sm *SecretMerger) Update(sec *dependencies.Secret) (*dependencies.Secret, error) {
	return sm.Create(sec)
}

func (sm *SecretMerger) Delete(name string) error {
	delete(sm.inMemory, name)
	return nil
}

func (sm *SecretMerger) Get(name string) (*dependencies.Secret, error) {
	sec, ok := sm.inMemory[name]
	if !ok {
		return sm.SecretStorage.Get(name)
	}
	return sec, nil
}

func (sm *SecretMerger) List() ([]*dependencies.Secret, error) {
	baseList, err := sm.SecretStorage.List()
	if err != nil {
		return nil, err
	}
	for _, sec := range sm.inMemory {
		baseList = append(baseList, sec)
	}
	return baseList, nil
}

func (sm *SecretMerger) Watch(handlers ... dependencies.SecretEventHandler) (*storage.Watcher, error) {
	baseWatch, err := sm.SecretStorage.Watch(&dependencies.SecretEventHandlerFuncs{
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
	sync := func() error {
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
				if err := sync(); err != nil {
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
