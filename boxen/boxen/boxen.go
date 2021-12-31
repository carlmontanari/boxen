package boxen

import (
	"log"
	"sync"

	"github.com/carlmontanari/boxen/boxen/config"
	"github.com/carlmontanari/boxen/boxen/logging"
	"github.com/carlmontanari/boxen/boxen/platforms"
	"github.com/carlmontanari/boxen/boxen/util"
)

// args is a simple struct for holding Boxen option arguments.
type args struct {
	logger *logging.Instance
	config string
}

// Boxen is the main "manager"/instance containing all the configuration data and maps of instances.
type Boxen struct {
	ConfigPath    string
	Config        *config.Config
	configLock    *sync.Mutex
	Instances     map[string]platforms.Platform
	instancesLock *sync.Mutex
	Logger        *logging.Instance
}

// NewBoxen returns an instance of Boxen with any provided Option applied.
func NewBoxen(opts ...Option) (*Boxen, error) {
	b := &Boxen{
		Instances:     map[string]platforms.Platform{},
		instancesLock: &sync.Mutex{},
		configLock:    &sync.Mutex{},
	}

	a := &args{}

	for _, o := range opts {
		err := o(a)

		if err != nil {
			return nil, err
		}
	}

	if a.logger != nil {
		b.Logger = a.logger
	} else {
		var err error

		b.Logger, err = logging.NewInstance(log.Print)

		if err != nil {
			return nil, err
		}
	}

	if a.config != "" {
		cp, err := util.ResolveFile(a.config)
		if err != nil {
			return nil, err
		}

		b.ConfigPath = cp

		cfg, err := config.NewConfigFromFile(b.ConfigPath)
		if err != nil {
			return nil, err
		}

		b.Config = cfg
	}

	return b, nil
}

// modifyInstanceMap is a simple method accepting a function f to write to the Instances map behind
// a simple sync.Mutex lock. This method is necessary due to the start and stop operations spawning
// goroutines for each instance provided by the user. Realistically this would *probably* never be
// a problem, but running things with -race flag definitely indicated potential issues so here we
// are!
func (b *Boxen) modifyInstanceMap(f func()) {
	b.instancesLock.Lock()
	defer b.instancesLock.Unlock()

	f()
}
