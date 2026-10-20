package agentmeta

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

// Methods retains author order without changing the manifest's object form.
type Methods map[string]Method

func (m *Methods) UnmarshalJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	token, err := d.Token()
	if err != nil {
		return err
	}
	if token != json.Delim('{') {
		return fmt.Errorf("methods must be an object")
	}
	*m = Methods{}
	for d.More() {
		token, err = d.Token()
		if err != nil {
			return err
		}
		name := token.(string)
		if _, exists := (*m)[name]; exists {
			return fmt.Errorf("duplicate method %q", name)
		}
		var method Method
		if err = d.Decode(&method); err != nil {
			return err
		}
		method.order = len(*m)
		(*m)[name] = method
	}
	_, err = d.Token()
	return err
}
func (m Methods) names() []string {
	names := keys(m)
	sort.SliceStable(names, func(i, j int) bool { return m[names[i]].order < m[names[j]].order })
	return names
}
func (m Methods) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, name := range m.names() {
		if i > 0 {
			b.WriteByte(',')
		}
		key, _ := json.Marshal(name)
		b.Write(key)
		b.WriteByte(':')
		value, err := json.Marshal(m[name])
		if err != nil {
			return nil, err
		}
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
func (m Manifest) resolveArgument(a Argument) (Argument, error) {
	if a.Help != "" {
		return a, nil
	}
	shared, ok := m.Arguments[a.Name]
	if !ok {
		return a, fmt.Errorf("argument %q needs help or a shared definition", a.Name)
	}
	a.Help = shared.Help
	if len(a.Type) == 0 {
		a.Type = shared.Type
	}
	if len(a.Default) == 0 {
		a.Default = shared.Default
	}
	a.shared = true
	return a, nil
}
