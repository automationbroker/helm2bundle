package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"path"
	"text/template"
)

const dockerfileTemplate string = `FROM mhrivnak/helm-bundle-base

LABEL "com.redhat.apb.spec"=\
""

COPY {{.TarfileName}} /opt/chart.tgz

ENTRYPOINT ["entrypoint.sh"]
`

// APB represents an apb.yml file
type APB struct {
	Version     string            `yaml:"version"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Bindable    bool              `yaml:"bindable"`
	Async       string            `yaml:"async"`
	Metadata    map[string]string `yaml:"metadata"`
	Plans       []Plan            `yaml:"plans"`
}

type Plan struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Free        bool                   `yaml:"free"`
	Metadata    map[string]interface{} `yaml:"metadata"`
	Parameters  []Parameter            `yaml:"parameters"`
}

type Parameter struct {
	Name        string `yaml:"name"`
	Title       string `yaml:"title"`
	Type        string `yaml:"type"`
	DisplayType string `yaml:"display_type"`
	Default     string `yaml:"default"`
}

func NewAPB(v TValues) *APB {
	parameter := Parameter{
		Name:        "values",
		Title:       "Values",
		Type:        "string",
		DisplayType: "textarea",
		Default:     v.Values,
	}
	plan := Plan{
		Name:        "default",
		Description: fmt.Sprintf("Deploys helm chart %s", v.Name),
		Free:        true,
		Metadata:    make(map[string]interface{}),
		Parameters:  []Parameter{parameter},
	}
	apb := APB{
		Version:     "1.0",
		Name:        fmt.Sprintf("%s-apb", v.Name),
		Description: v.Description,
		Bindable:    false,
		Async:       "optional",
		Metadata: map[string]string{
			"displayName":                    fmt.Sprintf("%s (helm bundle)", v.Name),
			"console.openshift.io/iconClass": fmt.Sprintf("icon-%s", v.Name), // no guarantee it exists, but worth a shot
		},
		Plans: []Plan{plan},
	}
	return &apb
}

// TValues holds data that will be used to create the Dockerfile and apb.yml
type TValues struct {
	Name        string
	Description string
	TarfileName string
	Values      string // the entire contents of the chart's values.yaml file
}

// Chart hold data that is parsed from a helm chart's Chart.yaml file.
type Chart struct {
	Description string
	Name        string
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "helm2bundle CHARTFILE",
		Short: "Packages a helm chart as a Service Bundle",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filename := args[0]

			values, err := getTValues(filename)
			if err != nil {
				fmt.Println(err.Error())
				panic("could not get values from helm chart")
			}

			err = writeApbYaml(values)
			if err != nil {
				fmt.Println(err.Error())
				panic("could not render template")
			}
			err = writeDockerfile(values)
			if err != nil {
				fmt.Println(err.Error())
				panic("could not render template")
			}
		},
	}

	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err.Error())
		panic("could not execute command")
	}
}

func writeApbYaml(v TValues) error {
	apb := NewAPB(v)
	data, err := yaml.Marshal(apb)
	if err != nil {
		return err
	}

	f, err := os.Create("apb.yml")
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func writeDockerfile(v TValues) error {
	t, err := template.New("Dockerfile").Parse(dockerfileTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create("Dockerfile")
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, v)
}

// getTValues opens the helm chart tarball to 1) retrieve Chart.yaml so it can
// be parsed, and 2) retrieve the entire contents of values.yaml.
func getTValues(filename string) (TValues, error) {
	file, err := os.Open(filename)
	if err != nil {
		return TValues{}, err
	}
	defer file.Close()

	uncompressed, err := gzip.NewReader(file)
	if err != nil {
		return TValues{}, err
	}

	tr := tar.NewReader(uncompressed)
	var chart Chart
	var values string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return TValues{}, errors.New("Chart.yaml not found in archive")
		}
		if err != nil {
			return TValues{}, err
		}

		chartMatch, err := path.Match("*/Chart.yaml", hdr.Name)
		valuesMatch, err := path.Match("*/values.yaml", hdr.Name)
		if err != nil {
			return TValues{}, err
		}
		if chartMatch {
			chart, err = parseChart(tr)
			if err != nil {
				return TValues{}, err
			}
		}
		if valuesMatch {
			data, err := ioutil.ReadAll(tr)
			if err != nil {
				return TValues{}, err
			}
			values = string(data)
		}
		if len(values) > 0 && len(chart.Name) > 0 {
			break
		}
	}
	if len(values) > 0 && len(chart.Name) > 0 {
		return TValues{
			Name:        chart.Name,
			Description: chart.Description,
			TarfileName: filename,
			Values:      values,
		}, nil
	}
	return TValues{}, errors.New("Could not find both Chart.yaml and values.yaml")
}

// parseChart parses the Chart.yaml file for data that is needed when creating
// a service bundle.
func parseChart(source io.Reader) (Chart, error) {
	c := Chart{}

	data, err := ioutil.ReadAll(source)
	if err != nil {
		return c, err
	}

	err = yaml.Unmarshal(data, &c)
	if err != nil {
		return c, err
	}

	return c, nil
}
