package kconfig

import (
	"fmt"
	"github.com/enfabrica/enkit/lib/kflags"
	"github.com/enfabrica/enkit/lib/multierror"
	"sync"
)

type ParamIndex map[string][]Retriever

type NamespaceAugmenter struct {
	index map[string]ParamIndex

	wg   sync.WaitGroup
	lock sync.RWMutex // Protects errs below, but also access to visited flags (which may not support concurrent access).
	errs []error      // Collects errors generated asynchronously by the downloader. Use only under lock.
}

type Factory func(*Parameter) (Retriever, error)

func NewNamespaceAugmenter(commands []Namespace, factory Factory) (*NamespaceAugmenter, error) {
	ci := &NamespaceAugmenter{index: map[string]ParamIndex{}}
	errs := []error{}
	for _, cmd := range commands {
		_, found := ci.index[cmd.Name]
		if found {
			errs = append(errs, fmt.Errorf("command %s - defined multiple times in config - will only consider the first definition", cmd.Name))
			continue
		}

		pi := ParamIndex{}
		for dx, def := range cmd.Default {
			params, _ := pi[def.Name]

			retriever, err := factory(&cmd.Default[dx])
			if err != nil {
				errs = append(errs, err)
				continue
			}

			params = append(params, retriever)
			pi[def.Name] = params
		}
		ci.index[cmd.Name] = pi
	}

	return ci, multierror.New(errs)
}

func (c *NamespaceAugmenter) Visit(namespace string, flag kflags.Flag) (bool, error) {
	paramIndex, found := c.index[namespace]
	if !found {
		return false, nil
	}

	params, found := paramIndex[flag.Name()]
	if !found {
		return false, nil
	}

	setter := func(origin, value string, err error) {
		c.lock.Lock()
		defer c.lock.Unlock()

		c.wg.Done()
		if err != nil {
			c.errs = append(c.errs, err)
			return
		}
		if err := flag.SetContent(origin, []byte(value)); err != nil {
			c.errs = append(c.errs, fmt.Errorf("could not set flag '%s', value '%s' caused %w", flag.Name(), value, err))
		}
	}

	for _, p := range params {
		c.wg.Add(1)
		p.Retrieve(setter)
	}

	return true, nil
}

func (c *NamespaceAugmenter) Done() error {
	c.wg.Wait()

	// Not necessary - all downloads have completed by now. Here in case tsan is not smart enough.
	defer c.lock.RUnlock()
	c.lock.RLock()
	return multierror.New(c.errs)
}
