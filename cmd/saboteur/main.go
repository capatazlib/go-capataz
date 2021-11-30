package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/capatazlib/go-capataz/saboteur/api"
)

var hostname string
var contentType = "application/json"

func main() {
	app := cli.NewApp()
	app.Name = "saboteur"
	app.Commands = []*cli.Command{
		{
			Name:    "interactive",
			Aliases: []string{"i"},
			Action:  interactive,
			Flags: []cli.Flag{
				&cli.DurationFlag{
					Name:        "refresh",
					Value:       time.Second * 5,
					DefaultText: "5s",
				},
			},
		},
		{
			Name: "nodes",
			Action: func(c *cli.Context) error {
				nodes, err := listNodes()
				if err == nil {
					fmt.Printf("%+v\n", nodes)
				}
				return err
			},
		},
		{
			Name: "plans",
			Action: func(c *cli.Context) error {
				plans, err := listPlans()
				if err == nil {
					fmt.Printf("%+v\n", plans)
				}
				return err
			},
		},
		{
			Name:   "add",
			Action: add,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "name",
					Required: true,
				},
				&cli.StringFlag{
					Name:     "subtree",
					Required: true,
				},
				&cli.DurationFlag{
					Name:     "duration",
					Required: true,
				},
				&cli.DurationFlag{
					Name:     "period",
					Required: true,
				},
				&cli.IntFlag{
					Name:     "attempts",
					Required: true,
				},
			},
		},
		{
			Name:   "remove",
			Action: remove,
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "name",
					Required: true,
				},
			},
		},
		{
			Name: "start",
			Action: func(c *cli.Context) error {
				err := start(c.String("name"))
				if err == nil {
					fmt.Println("started plan")
				}
				return err
			},
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "name",
					Required: true,
				},
			},
		},
		{
			Name: "stop",
			Action: func(c *cli.Context) error {
				err := stop(c.String("name"))
				if err == nil {
					fmt.Println("stopped plan")
				}
				return err
			},
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:     "name",
					Required: true,
				},
			},
		},
	}
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:        "host",
			Value:       "http://localhost:4784",
			Usage:       "saboteur server to connect to",
			Destination: &hostname,
			EnvVars:     []string{"SABOTEUR_URL"},
		},
	}
	app.Run(os.Args)
}

func listNodes() (api.Nodes, error) {
	nodes := api.Nodes{}
	resp, err := http.Get(fmt.Sprintf("%s/nodes", hostname))
	if err := checkResp(err, resp, http.StatusOK, "list nodes"); err != nil {
		return nodes, err
	}
	err = json.NewDecoder(resp.Body).Decode(&nodes)
	if err != nil {
		return nodes, errorf("failed to decode nodes: %s", err)
	}
	return nodes, nil
}

func listPlans() (api.Plans, error) {
	plans := api.Plans{}
	resp, err := http.Get(fmt.Sprintf("%s/plans", hostname))
	if err := checkResp(err, resp, http.StatusOK, "list plans"); err != nil {
		return plans, err
	}
	err = json.NewDecoder(resp.Body).Decode(&plans)
	if err != nil {
		return plans, errorf("failed to decode plans: %s", err)
	}
	return plans, nil
}

func remove(c *cli.Context) error {
	httpClient := &http.Client{}
	req, err := http.NewRequest(
		"DELETE",

		fmt.Sprintf(
			"%s/plans/%s",
			hostname,
			c.String("name"),
		),
		nil,
	)
	if err != nil {
		return errorf("failed to build deletion request: %s", err)
	}
	resp, err := httpClient.Do(req)
	if err := checkResp(err, resp, http.StatusNoContent, "remove plan"); err != nil {
		return err
	}
	return nil
}

func add(c *cli.Context) error {
	var err error
	plan := api.Plan{
		Name:        c.String("name"),
		SubtreeName: c.String("subtree"),
		Duration:    c.Duration("duration"),
		Period:      c.Duration("period"),
		Attempts:    uint32(c.Int("attempts")),
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return errorf("failed to encode plan: %s", err)
	}
	resp, err := http.Post(
		fmt.Sprintf("%s/plans", hostname),
		contentType,
		bytes.NewReader(data),
	)
	if err := checkResp(err, resp, http.StatusNoContent, "create plan"); err != nil {
		return err
	}
	defer resp.Body.Close()
	fmt.Println("plan created")
	return nil
}

func start(name string) error {
	resp, err := http.Post(
		fmt.Sprintf(
			"%s/plans/%s/start",
			hostname,
			name,
		),
		contentType,
		nil,
	)
	if err := checkResp(err, resp, http.StatusNoContent, "start plan"); err != nil {
		return err
	}
	return nil
}

func stop(name string) error {
	resp, err := http.Post(
		fmt.Sprintf(
			"%s/plans/%s/stop",
			hostname,
			name,
		),
		contentType,
		nil,
	)
	if err := checkResp(err, resp, http.StatusNoContent, "stop plan"); err != nil {
		return err
	}
	return nil
}

func errorf(m string, args ...interface{}) error {
	return cli.Exit(fmt.Sprintf(m, args...), 1)
}

func checkResp(err error, resp *http.Response, expectedCode int, caller string) error {
	if err != nil {
		return errorf("failed to %s: %s", caller, err)
	}
	if resp.StatusCode != expectedCode {
		e := api.Error{}
		err := json.NewDecoder(resp.Body).Decode(&e)
		if err != nil {
			e.Error = "unknown error"
		}
		return errorf("failed to %s: %s", caller, e.Error)
	}
	return nil
}
