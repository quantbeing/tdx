package command

type Command interface {
	BuildRequest() ([]byte, error)
	ParseResponse(body []byte) (any, error)
	Operation() string
}

type UnsupportedCommand struct {
	Name string
}

func (c UnsupportedCommand) BuildRequest() ([]byte, error) {
	return nil, ErrUnsupported{Operation: c.Name}
}

func (c UnsupportedCommand) ParseResponse([]byte) (any, error) {
	return nil, ErrUnsupported{Operation: c.Name}
}

func (c UnsupportedCommand) Operation() string {
	return c.Name
}

type ErrUnsupported struct {
	Operation string
}

func (e ErrUnsupported) Error() string {
	return "tdx command unsupported: " + e.Operation
}
