package format

import "fmt"

/** Holds every known module and picks the right one for a file */
type Registry struct {
	modules  map[string]Module
	order    []string
	fallback Module
}

func NewRegistry(fallback Module) *Registry {
	return &Registry{
		modules:  map[string]Module{fallback.ID(): fallback},
		fallback: fallback,
	}
}

func (r *Registry) Register(m Module) error {
	if _, taken := r.modules[m.ID()]; taken {
		return fmt.Errorf("format module %q is already registered", m.ID())
	}
	r.modules[m.ID()] = m
	r.order = append(r.order, m.ID())
	return nil
}

func (r *Registry) ByID(id string) (Module, bool) {
	m, ok := r.modules[id]
	return m, ok
}

/** Return the first module that recognises the file, or the fallback */
func (r *Registry) Detect(filename string, head []byte) Module {
	for _, id := range r.order {
		if r.modules[id].Detect(filename, head) {
			return r.modules[id]
		}
	}
	return r.fallback
}

func (r *Registry) CanEdit(id string) bool {
	m, ok := r.modules[id]
	if !ok {
		return false
	}
	_, editable := m.(Editor)
	return editable
}
