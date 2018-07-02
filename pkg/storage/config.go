package storage

import (
	"github.com/solo-io/gloo/pkg/storage"
	"github.com/solo-io/gloo/pkg/api/types/v1"
	"sort"
	"github.com/pkg/errors"
	"github.com/solo-io/gloo/pkg/log"
	"sync"
)

type PartialInMemoryConfig struct {
	gloo      storage.Interface
	upstreams *inMemoryUpstreams
	roles     *inMemoryRoles
}

var _ storage.Interface = &PartialInMemoryConfig{}

func NewPartialInMemoryConfig(gloo storage.Interface) *PartialInMemoryConfig {
	return &PartialInMemoryConfig{
		gloo:      gloo,
		upstreams: newInMemoryUpstreams(),
		roles:     newInMemoryRoles(),
	}
}

func (s *PartialInMemoryConfig) V1() storage.V1 {
	return s
}

func (s *PartialInMemoryConfig) Register() error {
	return s.gloo.V1().Register()
}

func (s *PartialInMemoryConfig) Upstreams() storage.Upstreams {
	return s.upstreams
}

func (s *PartialInMemoryConfig) VirtualServices() storage.VirtualServices {
	return s.gloo.V1().VirtualServices()
}

func (s *PartialInMemoryConfig) Roles() storage.Roles {
	return s.roles
}

/**************************************
	Upstreams
 *************************************/

type inMemoryUpstreams struct {
	store *inMemoryStore
}

func newInMemoryUpstreams() *inMemoryUpstreams {
	return &inMemoryUpstreams{
		store: newInMemoryStore(),
	}
}

func (s *inMemoryUpstreams) Create(obj *v1.Upstream) (*v1.Upstream, error) {
	out, err := s.store.Create(obj)
	if err != nil {
		return nil, err
	}
	return out.(*v1.Upstream), nil
}

func (s *inMemoryUpstreams) Update(obj *v1.Upstream) (*v1.Upstream, error) {
	out, err := s.store.Update(obj)
	if err != nil {
		return nil, err
	}
	return out.(*v1.Upstream), nil
}

func (s *inMemoryUpstreams) Delete(name string) error {
	return s.store.Delete(name)
}

func (s *inMemoryUpstreams) Get(name string) (*v1.Upstream, error) {
	out, err := s.store.Get(name)
	if err != nil {
		return nil, err
	}
	return out.(*v1.Upstream), nil
}

func (s *inMemoryUpstreams) List() ([]*v1.Upstream, error) {
	objects, err := s.store.List()
	if err != nil {
		return nil, err
	}
	var out []*v1.Upstream
	for _, obj := range objects {
		out = append(out, obj.(*v1.Upstream))
	}
	return out, nil
}

func (s *inMemoryUpstreams) Watch(handlers ... storage.UpstreamEventHandler) (*storage.Watcher, error) {
	return s.store.Watch(handlers, nil)
}

/**************************************
	Roles
 *************************************/

type inMemoryRoles struct {
	store *inMemoryStore
}

func newInMemoryRoles() *inMemoryRoles {
	return &inMemoryRoles{
		store: newInMemoryStore(),
	}
}

func (s *inMemoryRoles) Create(obj *v1.Role) (*v1.Role, error) {
	out, err := s.store.Create(obj)
	if err != nil {
		return nil, err
	}
	return out.(*v1.Role), nil
}

func (s *inMemoryRoles) Update(obj *v1.Role) (*v1.Role, error) {
	out, err := s.store.Update(obj)
	if err != nil {
		return nil, err
	}
	return out.(*v1.Role), nil
}

func (s *inMemoryRoles) Delete(name string) error {
	return s.store.Delete(name)
}

func (s *inMemoryRoles) Get(name string) (*v1.Role, error) {
	out, err := s.store.Get(name)
	if err != nil {
		return nil, err
	}
	return out.(*v1.Role), nil
}

func (s *inMemoryRoles) List() ([]*v1.Role, error) {
	objects, err := s.store.List()
	if err != nil {
		return nil, err
	}
	var out []*v1.Role
	for _, obj := range objects {
		out = append(out, obj.(*v1.Role))
	}
	return out, nil
}

func (s *inMemoryRoles) Watch(handlers ... storage.RoleEventHandler) (*storage.Watcher, error) {
	return s.store.Watch(nil, handlers)
}

/**************************************
	Base
 *************************************/

type inMemoryStore struct {
	lock    sync.RWMutex
	objects map[string]v1.ConfigObject
	updates chan struct{}
}

func newInMemoryStore() *inMemoryStore {
	return &inMemoryStore{
		objects: make(map[string]v1.ConfigObject),
		updates: make(chan struct{}, 10),
	}
}

func (s *inMemoryStore) updated() {
	go func() {
		s.updates <- struct{}{}
	}()
}

func (s *inMemoryStore) Create(obj v1.ConfigObject) (v1.ConfigObject, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, exists := s.objects[obj.GetName()]; exists {
		return nil, storage.NewAlreadyExistsErr(errors.Errorf("%s already exists", obj.GetName()))
	}
	s.objects[obj.GetName()] = obj
	s.updated()
	return obj, nil
}

func (s *inMemoryStore) Update(obj v1.ConfigObject) (v1.ConfigObject, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, exists := s.objects[obj.GetName()]; !exists {
		return nil, errors.Errorf("%s does not exist", obj.GetName())
	}
	s.objects[obj.GetName()] = obj
	s.updated()
	return obj, nil
}

func (s *inMemoryStore) Delete(name string) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, exists := s.objects[name]; !exists {
		return errors.Errorf("%s does not exist", name)
	}
	delete(s.objects, name)
	s.updated()
	return nil
}

func (s *inMemoryStore) Get(name string) (v1.ConfigObject, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	obj, exists := s.objects[name]
	if !exists {
		return nil, errors.Errorf("%s does not exist", name)
	}
	return obj, nil
}

func (s *inMemoryStore) List() ([]v1.ConfigObject, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var objs []v1.ConfigObject
	for _, obj := range s.objects {
		objs = append(objs, obj)
	}
	sort.SliceStable(objs, func(i, j int) bool {
		return objs[i].GetName() < objs[j].GetName()
	})
	return objs, nil
}

func (s *inMemoryStore) Watch(upstreamHandlers []storage.UpstreamEventHandler, roleHandlers []storage.RoleEventHandler) (*storage.Watcher, error) {
	if len(upstreamHandlers) > 0 && len(roleHandlers) > 0 {
		return nil, errors.Errorf("internal error: can only specify role event handlers or upstream event handlers, "+
			"got %v upstream and %v role event handlers", len(upstreamHandlers), len(roleHandlers))
	}
	return storage.NewWatcher(func(stop <-chan struct{}, errs chan error) {
		for {
			select {
			case <-s.updates:
				objs, err := s.List()
				if err != nil {
					log.Warnf("failed to list config objects: %v", err)
					continue
				}
				switch {
				case len(upstreamHandlers) > 0:
					var upstreams []*v1.Upstream
					for _, obj := range objs {
						upstreams = append(upstreams, obj.(*v1.Upstream))
					}
					for _, h := range upstreamHandlers {
						h.OnUpdate(upstreams, nil)
					}
				case len(roleHandlers) > 0:
					var roles []*v1.Role
					for _, obj := range objs {
						roles = append(roles, obj.(*v1.Role))
					}
					for _, h := range roleHandlers {
						h.OnUpdate(roles, nil)
					}
				default:
					// no handlers specified
					return
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
