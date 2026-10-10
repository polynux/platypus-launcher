package social

import "slices"

// Plugins lists the plugins with a subscription, in no particular order.
func (m *Manager) Plugins() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := make([]string, 0, len(m.subs))
	for id := range m.subs {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
