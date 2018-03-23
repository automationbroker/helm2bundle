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

const dockerfileTemplate string = `FROM ansibleplaybookbundle/helm-bundle-base

LABEL "com.redhat.apb.spec"=\
""

COPY {{.TarfileName}} /opt/chart.tgz

ENTRYPOINT ["entrypoint.sh"]
`

const apbYml string = "apb.yml"
const dockerfile string = "Dockerfile"

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

// NewAPB returns a pointer to a new APB that has been populated with the
// passed-in data.
func NewAPB(v TarValues) *APB {
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

// TarValues holds data that will be used to create the Dockerfile and apb.yml
type TarValues struct {
	Name        string
	Description string
	TarfileName string
	Values      string // the entire contents of the chart's values.yaml file
}

// Chart holds data that is parsed from a helm chart's Chart.yaml file.
type Chart struct {
	Description string
	Name        string
}

func main() {
	// forceArg is true when the user specifies --force, and it indicates that
	// it is ok to replace existing files.
	var forceArg bool

	var rootCmd = &cobra.Command{
		Use:   "helm2bundle CHARTFILE",
		Short: "Packages a helm chart as a Service Bundle",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if forceArg == false {
				// fail if one of the files already exists
				exists, err := fileExists()
				if err != nil {
					fmt.Println(err.Error())
					fmt.Println("could not get values from helm chart")
					os.Exit(1)
				}
				if exists {
					fmt.Printf("use --force to overwrite existing %s and/or %s\n", dockerfile, apbYml)
					os.Exit(1)
				}
			}

			filename := args[0]

			values, err := getTarValues(filename)
			if err != nil {
				fmt.Println(err.Error())
				fmt.Println("could not get values from helm chart")
				os.Exit(1)
			}

			err = writeApbYaml(values)
			if err != nil {
				fmt.Println(err.Error())
				fmt.Println("could not render template")
				os.Exit(1)
			}
			err = writeDockerfile(values)
			if err != nil {
				fmt.Println(err.Error())
				fmt.Println("could not render template")
				os.Exit(1)
			}
		},
	}

	rootCmd.PersistentFlags().BoolVarP(&forceArg, "force", "f", false, "force overwrite of existing files")

	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err.Error())
		fmt.Println("could not execute command")
		os.Exit(1)
	}
}

// fileExists returns true if either apb.yml or Dockerfile exist in the working
// directory, else false
func fileExists() (bool, error) {
	for _, filename := range []string{apbYml, dockerfile} {
		_, err := os.Stat(filename)
		if err == nil {
			// file exists
			return true, nil
		}
		if !os.IsNotExist(err) {
			// error determining if file exists
			return false, err
		}
	}
	// neither file exists
	return false, nil
}

// writeApbYaml creates a new file named "apb.yml" in the current working
// directory that can be used to build a service bundle.
func writeApbYaml(v TarValues) error {
	apb := NewAPB(v)
	data, err := yaml.Marshal(apb)
	if err != nil {
		return err
	}

	f, err := os.Create(apbYml)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

// writeDockerfile creates a new file named "Dockerfile" in the current working
// directory that can be used to build a service bundle.
func writeDockerfile(v TarValues) error {
	t, err := template.New(dockerfile).Parse(dockerfileTemplate)
	if err != nil {
		return err
	}

	f, err := os.Create(dockerfile)
	if err != nil {
		return err
	}
	defer f.Close()

	return t.Execute(f, v)
}

// getTarValues opens the helm chart tarball to 1) retrieve Chart.yaml so it can
// be parsed, and 2) retrieve the entire contents of values.yaml.
func getTarValues(filename string) (TarValues, error) {
	file, err := os.Open(filename)
	if err != nil {
		return TarValues{}, err
	}
	defer file.Close()

	uncompressed, err := gzip.NewReader(file)
	if err != nil {
		return TarValues{}, err
	}

	tr := tar.NewReader(uncompressed)
	var chart Chart
	var values string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return TarValues{}, errors.New("Chart.yaml not found in archive")
		}
		if err != nil {
			return TarValues{}, err
		}

		chartMatch, err := path.Match("*/Chart.yaml", hdr.Name)
		if err != nil {
			return TarValues{}, err
		}
		valuesMatch, err := path.Match("*/values.yaml", hdr.Name)
		if err != nil {
			return TarValues{}, err
		}
		if chartMatch {
			chart, err = parseChart(tr)
			if err != nil {
				return TarValues{}, err
			}
		}
		if valuesMatch {
			data, err := ioutil.ReadAll(tr)
			if err != nil {
				return TarValues{}, err
			}
			values = string(data)
		}
		if len(values) > 0 && len(chart.Name) > 0 {
			break
		}
	}
	if len(values) > 0 && len(chart.Name) > 0 {
		return TarValues{
			Name:        chart.Name,
			Description: chart.Description,
			TarfileName: filename,
			Values:      values,
		}, nil
	}
	return TarValues{}, errors.New("Could not find both Chart.yaml and values.yaml")
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
