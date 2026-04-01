package picker

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/flagifyhq/cli/internal/api"
)

func PickWorkspace(client *api.Client) (*api.Workspace, error) {
	workspaces, err := client.ListWorkspaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}
	if len(workspaces) == 0 {
		return nil, fmt.Errorf("no workspaces found")
	}
	if len(workspaces) == 1 {
		return &workspaces[0], nil
	}

	opts := make([]huh.Option[int], len(workspaces))
	for i, ws := range workspaces {
		opts[i] = huh.NewOption(fmt.Sprintf("%s (%s)", ws.Name, ws.Slug), i)
	}

	var selected int
	err = huh.NewSelect[int]().
		Title("Select a workspace").
		Options(opts...).
		Value(&selected).
		Run()
	if err != nil {
		return nil, err
	}
	return &workspaces[selected], nil
}

func PickProject(client *api.Client, workspaceID string) (*api.Project, error) {
	projects, err := client.ListProjects(workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("no projects found in workspace")
	}
	if len(projects) == 1 {
		return &projects[0], nil
	}

	opts := make([]huh.Option[int], len(projects))
	for i, p := range projects {
		opts[i] = huh.NewOption(fmt.Sprintf("%s (%s)", p.Name, p.Slug), i)
	}

	var selected int
	err = huh.NewSelect[int]().
		Title("Select a project").
		Options(opts...).
		Value(&selected).
		Run()
	if err != nil {
		return nil, err
	}
	return &projects[selected], nil
}

func PickFlag(flags []api.Flag, flagType string) (*api.Flag, error) {
	var filtered []api.Flag
	for _, f := range flags {
		if flagType == "" || f.Type == flagType {
			filtered = append(filtered, f)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no %s flags found in project", flagType)
	}
	if len(filtered) == 1 {
		return &filtered[0], nil
	}

	opts := make([]huh.Option[int], len(filtered))
	for i, f := range filtered {
		label := fmt.Sprintf("%s (%s)", f.Key, f.Name)
		opts[i] = huh.NewOption(label, i)
	}

	var selected int
	err := huh.NewSelect[int]().
		Title("Select a flag").
		Options(opts...).
		Value(&selected).
		Run()
	if err != nil {
		return nil, err
	}
	return &filtered[selected], nil
}

func PickEnvironment(client *api.Client, projectID string) (*api.Environment, error) {
	project, err := client.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	envs := project.Environments
	if len(envs) == 0 {
		return nil, fmt.Errorf("no environments found in project")
	}
	if len(envs) == 1 {
		return &envs[0], nil
	}

	opts := make([]huh.Option[int], len(envs))
	for i, e := range envs {
		opts[i] = huh.NewOption(fmt.Sprintf("%s (%s)", e.Name, e.Key), i)
	}

	var selected int
	err = huh.NewSelect[int]().
		Title("Select an environment").
		Options(opts...).
		Value(&selected).
		Run()
	if err != nil {
		return nil, err
	}
	return &envs[selected], nil
}
