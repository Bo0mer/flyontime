package flyontime

type Command struct {
	Name      string
	Args      []string
	Job       *Job // Job which the command is targeted for (if any).
	Responses chan<- string
}

//go:generate counterfeiter . Commander

type Commander interface {
	Commands() <-chan *Command
}
