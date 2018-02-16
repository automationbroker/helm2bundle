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

const bundleTemplate string = `version: 1.0
name: {{.Name}}-apb
description: {{.Description}}
bindable: False
async: optional
metadata:
  displayName: {{.Name}}-helm
  imageURL: {{.ImageURL}}
plans:
  - name: default
    description: This default plan deploys helm chart {{.Name}}
    free: True
    metadata: {}
    parameters: []
`

type Values struct {
	Name        string
	Description string
	ImageURL    string
}

type Chart struct {
	Description string
	Name        string
	Icon        string
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "helm2bundle CHARTFILE",
		Short: "Packages a helm chart as a Service Bundle",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filename := args[0]

			t, err := template.New("bundle").Parse(bundleTemplate)
			if err != nil {
				fmt.Println(err.Error())
				panic("could not parse template")
			}

			values, err := getValues(filename)
			if err != nil {
				fmt.Println(err.Error())
				panic("could not get values from chart")
			}

			err = t.Execute(os.Stdout, values)
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

func getValues(filename string) (Values, error) {
	file, err := os.Open(filename)
	if err != nil {
		return Values{}, err
	}

	uncompressed, err := gzip.NewReader(file)
	if err != nil {
		return Values{}, err
	}

	tr := tar.NewReader(uncompressed)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return Values{}, errors.New("Chart.yaml not found in archive")
		}
		if err != nil {
			return Values{}, err
		}

		match, err := path.Match("*/Chart.yaml", hdr.Name)
		if err != nil {
			return Values{}, err
		}
		if match {
			chart, err := parseChart(tr)
			if err != nil {
				return Values{}, err
			}
			return Values{
				Name:        chart.Name,
				Description: chart.Description,
				ImageURL:    chart.Icon,
			}, nil
		}
	}
	return Values{}, errors.New("something went wrong")
}

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
