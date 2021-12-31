package boxen

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"strings"

	"github.com/carlmontanari/boxen/boxen"
)

type configTemplateArgs struct {
	Username          string
	Password          string
	SecondaryPassword string
}

// RenderInitialConfig renders the initial installation config template.
func (b *Boxen) RenderInitialConfig(
	name string,
) ([]string, error) {
	platformType := b.Config.Instances[name].PlatformType

	templateData := &configTemplateArgs{
		Username: b.Config.Instances[name].Credentials.Username,
		Password: b.Config.Instances[name].Credentials.Password,
	}

	t, err := template.ParseFS(
		boxen.Assets,
		fmt.Sprintf("assets/configs/%s.template", platformType),
	)

	envProfilePath := os.Getenv(
		fmt.Sprintf("BOXEN_%s_INITIAL_CONFIG_TEMPLATE", strings.ToUpper(platformType)),
	)
	if envProfilePath != "" {
		t, err = template.ParseFiles(envProfilePath)
	}

	if err != nil {
		return nil, err
	}

	var rendered bytes.Buffer

	err = t.Execute(&rendered, templateData)
	if err != nil {
		return nil, err
	}

	return strings.Split(rendered.String(), "\n"), nil
}
