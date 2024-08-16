package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"code.gitea.io/sdk/gitea"
	"github.com/alexflint/go-arg"
	"github.com/cornelk/gotokit/env"
	"github.com/cornelk/gotokit/log"
	"github.com/xanzy/go-gitlab"
)

var (
	GiteaAdminUser = "icepie"
)

type arguments struct {
	GitlabToken  string `arg:"--gitlabtoken,required" help:"token for GitLab API access"`
	GitlabServer string `arg:"--gitlabserver" help:"GitLab server URL with a trailing slash"`
	GiteaAdmin   string `arg:"--giteaadmin" help:"Gitea Admin User"`
	// GitlabProject string `arg:"--gitlabproject,required" help:"GitLab project name, use namespace/name"`
	GiteaToken  string `arg:"--giteatoken,required" help:"token for Gitea API access"`
	GiteaServer string `arg:"--giteaserver,required" help:"Gitea server URL"`
	// GiteaProject  string `arg:"--giteaproject" help:"Gitea project name, use namespace/name. defaults to GitLab project name"`
}

func (arguments) Description() string {
	return "Migrate labels, issues and milestones from GitLab to Gitea.\n"
}

type migrator struct {
	args   arguments
	logger *log.Logger

	userCache map[string]string

	gitlab *gitlab.Client
	// gitlabProjectID int

	// gitlabProjectCache map[string]int

	gitea *gitea.Client

	// giteaProjectID int64

	// gitteaProjectCache map[string]int

	// giteaRepo  string
	// giteaOwner string
}

func main() {
	args, err := readArguments()
	if err != nil {
		fmt.Printf("Reading arguments failed: %s\n", err)
		os.Exit(1)
	}

	if args.GiteaAdmin != "" {
		GiteaAdminUser = args.GiteaAdmin
	}

	logger, err := createLogger()
	if err != nil {
		fmt.Printf("Creating logger failed: %s\n", err)
		os.Exit(1)
	}

	m, err := newMigrator(args, logger)
	if err != nil {
		logger.Fatal("Creating migrator failed", log.Err(err))
	}

	err = m.migrateUsers()
	if err != nil {
		logger.Fatal("Creating migrator failed", log.Err(err))
	}

	logger.Info("Migrating users finished successfully")

	err = m.migrateOrgs()
	if err != nil {
		logger.Fatal("Creating migrator failed", log.Err(err))
	}

	logger.Info("Migrating orgs finished successfully")

	err = m.migrateRepo()
	if err != nil {
		logger.Fatal("Creating migrator failed", log.Err(err))
	}

	logger.Info("Migrating repo finished successfully")

	// if err := m.migrateProject(); err != nil {
	// 	m.logger.Fatal("Migrating the project failed", log.Err(err))
	// }

	m.logger.Info("Migration finished successfully")
}

func readArguments() (arguments, error) {
	var args arguments
	parser, err := arg.NewParser(arg.Config{}, &args)
	if err != nil {
		return arguments{}, fmt.Errorf("creating argument parser: %w", err)
	}

	if err = parser.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, arg.ErrHelp) || errors.Is(err, arg.ErrVersion) {
			parser.WriteHelp(os.Stdout)
			os.Exit(0)
		}

		return arguments{}, fmt.Errorf("parsing arguments: %w", err)
	}

	return args, nil
}

func createLogger() (*log.Logger, error) {
	cfg, err := log.ConfigForEnv(env.Development)
	if err != nil {
		return nil, fmt.Errorf("initializing log config: %w", err)
	}
	cfg.JSONOutput = false
	cfg.CallerInfo = false

	logger, err := log.NewWithConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("initializing logger: %w", err)
	}
	return logger, nil
}

// newMigrator returns a new creator object.
// It also tests that Gitlab and gitea can be reached.
func newMigrator(args arguments, logger *log.Logger) (*migrator, error) {
	m := &migrator{
		args:      args,
		logger:    logger,
		userCache: make(map[string]string, 0),
	}

	var err error
	m.gitlab, err = m.gitlabClient()
	if err != nil {
		return nil, err
	}

	m.gitea, err = m.giteaClient()
	if err != nil {
		return nil, err
	}

	return m, nil
}

// gitlabClient returns a new Gitlab client with the given command line parameters.
func (m *migrator) gitlabClient() (*gitlab.Client, error) {
	client, err := gitlab.NewClient(m.args.GitlabToken, gitlab.WithBaseURL(m.args.GitlabServer))
	if err != nil {
		return nil, fmt.Errorf("creating Gitlab client: %w", err)
	}

	// get the user status to check that the auth and connection works
	_, _, err = client.Users.CurrentUserStatus()
	if err != nil {
		return nil, fmt.Errorf("getting GitLab user status: %w", err)
	}

	// 暂时不需要

	// project, _, err := client.Projects.GetProject(m.args.GitlabProject, nil)
	// if err != nil {
	// 	return nil, fmt.Errorf("getting GitLab project info: %w", err)
	// }
	// m.gitlabProjectID = project.ID

	return client, nil
}

// giteaClient returns a new Gitea client with the given command line parameters.
func (m *migrator) giteaClient() (*gitea.Client, error) {
	client, err := gitea.NewClient(m.args.GiteaServer, gitea.SetToken(m.args.GiteaToken))
	if err != nil {
		return nil, fmt.Errorf("creating Gitea client: %w", err)
	}

	// get the user info to check that the auth and connection works
	_, _, err = client.GetMyUserInfo()
	if err != nil {
		return nil, fmt.Errorf("getting Gitea user info: %w", err)
	}

	// 暂时不需要

	// giteaProject := m.args.GiteaProject
	// if giteaProject == "" {
	// 	giteaProject = m.args.GitlabProject
	// }
	// sl := strings.Split(giteaProject, "/")
	// if len(sl) != 2 {
	// 	return nil, errors.New("wrong format of Gitea project name")
	// }
	// m.giteaOwner = sl[0]
	// m.giteaRepo = sl[1]

	// repo, _, err := client.GetRepo(m.giteaOwner, m.giteaRepo)
	// if err != nil {
	// 	return nil, fmt.Errorf("getting Gitea repo info: %w", err)
	// }
	// m.giteaProjectID = repo.ID

	return client, nil
}

// migrateProject migrates all supported aspects of a project.
func (m *migrator) migrateProject(gitlabProjectID int, giteaOwner string, giteaRepo string) error {
	m.logger.Info("Migrating milestones")
	if err := m.migrateMilestones(gitlabProjectID, giteaOwner, giteaRepo); err != nil {
		return fmt.Errorf("migrating milestones: %w", err)
	}

	m.logger.Info("Migrating labels")
	if err := m.migrateLabels(gitlabProjectID, giteaOwner, giteaRepo); err != nil {
		return fmt.Errorf("migrating labels: %w", err)
	}

	m.logger.Info("Migrating issues")
	if err := m.migrateIssues(gitlabProjectID, giteaOwner, giteaRepo); err != nil {
		return fmt.Errorf("migrating issues: %w", err)
	}
	return nil
}

// 迁移用户
func (m *migrator) migrateUsers() error {

	for page := 1; ; page++ {

		// 获取所有用户
		users, _, err := m.gitlab.Users.ListUsers(&gitlab.ListUsersOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		})
		if err != nil {
			return err
		}

		if len(users) == 0 {
			return nil
		}

		for _, user := range users {

			m.userCache[user.Username] = user.Username

			// 列出用户

			// fmt.Println(user)
			// fmt.Printf("user: %v\n", user)

			if user.Username == "ghost" {
				continue
			}

			m.logger.Info("Migrating user", log.String("username", user.Username))

			// 查找用户是否存在
			_, _, TheErr := m.gitea.GetUserInfo(user.Username)
			if TheErr == nil {
				m.logger.Info("Skipping user, already exists", log.String("username", user.Username))

				continue

				// // delete user
				// _, err := m.gitea.AdminDeleteUser(user.Username)
				// if err != nil {
				// 	return err
				// }
			}

			MustChangePassword := true

			var Visibility = gitea.VisibleTypePublic

			// // 创建用户
			giteaUser, _, err := m.gitea.AdminCreateUser(gitea.CreateUserOption{
				SourceID:           0,
				Email:              user.Email,
				Username:           user.Username,
				FullName:           user.Name,
				Password:           "FD12345678",
				MustChangePassword: &MustChangePassword,
				Visibility:         &Visibility,
			})

			if err != nil {
				return err
			}

			m.logger.Info("Created user", log.String("username", giteaUser.UserName))
		}
	}

	// return nil
}

// 迁移组织
func (m *migrator) migrateOrgs() error {

	AllAvailable := true

	for page := 1; ; page++ {

		// 获取所有组织
		orgs, _, err := m.gitlab.Groups.ListGroups(&gitlab.ListGroupsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
			AllAvailable: &AllAvailable,
		})
		if err != nil {
			return err
		}

		if len(orgs) == 0 {
			return nil
		}

		for _, org := range orgs {

			path := strings.ReplaceAll(org.FullPath, "/", "_")

			// fmt.Printf(path)

			// fmt.Printf("org: %v\n", org)

			m.logger.Info("Migrating org", log.String("org", path))

			// 查找组织是否存在
			_, _, TheErr := m.gitea.GetOrg(path)
			if TheErr == nil {
				m.logger.Info("Skipping org, already exists", log.String("org", path))
				continue
			}

			// 创建组织
			_, _, err := m.gitea.AdminCreateOrg(GiteaAdminUser, gitea.CreateOrgOption{
				Name:                      path,
				FullName:                  org.Name,
				Description:               org.Description,
				Website:                   org.WebURL,
				Visibility:                gitea.VisibleTypePublic,
				RepoAdminChangeTeamAccess: true,
			})
			if err != nil {
				return err
			}

			m.logger.Info("Created org", log.String("orgname", org.Name))
		}
	}

	// return nil
}

func (m *migrator) IsGitLabUser(username string) bool {

	_, ok := m.userCache[username]

	return ok
}

// 迁移仓库
func (m *migrator) migrateRepo() error {

	for page := 1; ; page++ {

		// 获取所有仓库
		repos, _, err := m.gitlab.Projects.ListProjects(&gitlab.ListProjectsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100}})
		if err != nil {
			return err
		}

		if len(repos) == 0 {
			return nil
		}

		for _, repo := range repos {
			// fmt.Printf("repo: %v\n", repo)

			// // 仓库名
			// // 所有

			// fmt.Printf("repoName: %v\n", repo.Path)

			// fmt.Printf("repoWithNamespace: %v\n", repo.PathWithNamespace)

			// fmt.Printf("repoBelong: %v\n", repo.Namespace.FullPath)

			// fmt.Printf("CloneAddr: %v\n", repo.HTTPURLToRepo)

			repoOwner := strings.ReplaceAll(repo.Namespace.FullPath, "/", "_")

			m.logger.Info("Migrating repo", log.String("repo", fmt.Sprintf("%s/%s", repoOwner, repo.Path)))

			// isGitLabUser := m.IsGitLabUser(repoOwner)

			// 判断仓库是否存在
			_, _, TheErr := m.gitea.GetRepo(repoOwner, repo.Path)
			if TheErr == nil {
				m.logger.Info("Skipping repo, already exists", log.String("repo", fmt.Sprintf("%s/%s", repoOwner, repo.Path)))
			} else {

				// if isGitLabUser {

				// 	_, _, err := m.gitea.MigrateRepo(gitea.MigrateRepoOption{
				// 		CloneAddr:      repo.HTTPURLToRepo,
				// 		RepoName:       repo.Path,
				// 		Service:        gitea.GitServiceGitlab,
				// 		AuthToken:      m.args.GitlabToken,
				// 		Mirror:         true,
				// 		Private:        true,
				// 		Labels:         true,
				// 		Issues:         true,
				// 		Wiki:           true,
				// 		Milestones:     true,
				// 		PullRequests:   true,
				// 		Releases:       true,
				// 		Description:    repo.Description,
				// 		MirrorInterval: "20m",
				// 		RepoOwner:      repo.Namespace.FullPath,
				// 	})

				// 	if err != nil {
				// 		return err
				// 	}

				// } else {

				_, _, err := m.gitea.MigrateRepo(gitea.MigrateRepoOption{
					CloneAddr:      repo.HTTPURLToRepo,
					RepoName:       repo.Path,
					Service:        gitea.GitServiceGitlab,
					AuthToken:      m.args.GitlabToken,
					Mirror:         true,
					Private:        true,
					Labels:         true,
					Issues:         true,
					Wiki:           true,
					Milestones:     true,
					PullRequests:   true,
					Releases:       true,
					Description:    repo.Description,
					MirrorInterval: "20m",
					RepoOwner:      repoOwner,
				})

				if err != nil {
					return err
				}
				// }

			}

			theErr := m.migrateProject(repo.ID, repoOwner, repo.Path)
			if theErr != nil {
				fmt.Printf("迁移repo额外数据失败: %v \n", err)
			}

			// continue

			// // 查找仓库是否存在
			// _, _, TheErr := m.gitea.GetRepo(repo.Name)
			// if TheErr == nil {
			// 	fmt.Printf("跳过repo: %v 已经存在\n", repo)
			// 	continue
			// }
		}

	}

	// return nil
}

// migrateMilestones does the active milestones migration.
func (m *migrator) migrateMilestones(gitlabProjectID int, giteaOwner string, giteaRepo string) error {
	existing, err := m.giteaMilestones(giteaOwner, giteaRepo)
	if err != nil {
		return err
	}

	state := "active"
	for page := 1; ; page++ {
		opt := &gitlab.ListMilestonesOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
			State: &state,
		}

		gitlabMilestones, _, err := m.gitlab.Milestones.ListMilestones(gitlabProjectID, opt, nil)
		if err != nil {
			return err
		}
		if len(gitlabMilestones) == 0 {
			return nil
		}

		for _, milestone := range gitlabMilestones {
			if _, ok := existing[milestone.Title]; ok {
				continue
			}

			var state gitea.StateType

			switch milestone.State {
			case "open":
				state = gitea.StateOpen
			case "opened":
				state = gitea.StateOpen
			case "close":
				state = gitea.StateClosed
			case "closed":
				state = gitea.StateClosed
			}

			o := gitea.CreateMilestoneOption{
				Title:       milestone.Title,
				Description: milestone.Description,
				Deadline:    (*time.Time)(milestone.DueDate),
				State:       state,
			}
			if _, _, err = m.gitea.CreateMilestone(giteaOwner, giteaRepo, o); err != nil {
				return err
			}
			m.logger.Info("Created milestone", log.String("title", o.Title))
		}
	}
}

// migrateLabels migrates all labels.
func (m *migrator) migrateLabels(gitlabProjectID int, giteaOwner string, giteaRepo string) error {
	existing, err := m.giteaLabels(giteaOwner, giteaRepo)
	if err != nil {
		return err
	}

	for page := 1; ; page++ {
		opt := &gitlab.ListLabelsOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
		}

		gitlabLabels, _, err := m.gitlab.Labels.ListLabels(gitlabProjectID, opt, nil)
		if err != nil {
			return err
		}
		if len(gitlabLabels) == 0 {
			return nil
		}

		for _, label := range gitlabLabels {
			if _, ok := existing[label.Name]; ok {
				continue
			}

			o := gitea.CreateLabelOption{
				Name:        label.Name,
				Description: label.Description,
				Color:       label.Color,
			}
			if _, _, err = m.gitea.CreateLabel(giteaOwner, giteaRepo, o); err != nil {
				return err
			}
			m.logger.Info("Created label",
				log.String("name", o.Name),
				log.String("color", o.Color),
			)
		}
	}
}

// migrateIssues migrates all open issues.
func (m *migrator) migrateIssues(gitlabProjectID int, giteaOwner string, giteaRepo string) error {
	giteaIssues, err := m.giteaIssues(giteaOwner, giteaRepo)
	if err != nil {
		return err
	}
	giteaMilestones, err := m.giteaMilestones(giteaOwner, giteaRepo)
	if err != nil {
		return err
	}
	giteaLabels, err := m.giteaLabels(giteaOwner, giteaRepo)
	if err != nil {
		return err
	}

	// state := "opened"
	for page := 1; ; page++ {
		opt := &gitlab.ListProjectIssuesOptions{
			ListOptions: gitlab.ListOptions{
				Page:    page,
				PerPage: 100,
			},
			// State: &state,
		}

		gitlabIssues, _, err := m.gitlab.Issues.ListProjectIssues(gitlabProjectID, opt, nil)
		if err != nil {
			return err
		}
		if len(gitlabIssues) == 0 {
			return nil
		}

		for _, issue := range gitlabIssues {

			if err = m.migrateIssue(giteaOwner, giteaRepo, issue, giteaMilestones, giteaLabels, giteaIssues); err != nil {
				return err
			}
		}
	}
}

// migrateIssue migrates a single issue.
func (m *migrator) migrateIssue(giteaOwner string, giteaRepo string, issue *gitlab.Issue, giteaMilestones map[string]*gitea.Milestone,
	giteaLabels map[string]*gitea.Label, giteaIssues map[string]*gitea.Issue) error {
	o := gitea.CreateIssueOption{
		Title:    issue.Title,
		Body:     issue.Description,
		Deadline: (*time.Time)(issue.DueDate),
	}

	if issue.Milestone != nil {
		milestone, ok := giteaMilestones[issue.Milestone.Title]
		if ok {
			o.Milestone = milestone.ID
		} else {
			m.logger.Error("Unknown milestone", log.String("milestone", issue.Milestone.Title))
		}
	}

	for _, l := range issue.Labels {
		label, ok := giteaLabels[l]
		if ok {
			o.Labels = append(o.Labels, label.ID)
		} else {
			m.logger.Error("Unknown label", log.String("label", l))
		}
	}

	existing, ok := giteaIssues[issue.Title]
	if !ok {
		if _, _, err := m.gitea.CreateIssue(giteaOwner, giteaRepo, o); err != nil {
			return err
		}
		m.logger.Info("Created issue", log.String("title", o.Title))
		return nil
	}

	var state gitea.StateType

	switch issue.State {
	case "open":
		state = gitea.StateOpen
	case "opened":
		state = gitea.StateOpen
	case "close":
		state = gitea.StateClosed
	case "closed":
		state = gitea.StateClosed

	}

	editOptions := gitea.EditIssueOption{
		Title:     o.Title,
		Body:      &o.Body,
		Milestone: &o.Milestone,
		Deadline:  o.Deadline,
		State:     &state,
	}
	if _, _, err := m.gitea.EditIssue(giteaOwner, giteaRepo, existing.Index, editOptions); err != nil {
		return err
	}
	labelOptions := gitea.IssueLabelsOption{
		Labels: o.Labels,
	}
	if _, _, err := m.gitea.ReplaceIssueLabels(giteaOwner, giteaRepo, existing.Index, labelOptions); err != nil {
		return err
	}

	m.logger.Info("Updated issue", log.String("title", o.Title))
	return nil
}

// giteaMilestones returns a map of all gitea milestones.
func (m *migrator) giteaMilestones(giteaOwner string, giteaRepo string) (map[string]*gitea.Milestone, error) {
	opt := gitea.ListMilestoneOption{
		State: "all",
	}
	giteaMilestones, _, err := m.gitea.ListRepoMilestones(giteaOwner, giteaRepo, opt)
	if err != nil {
		return nil, err
	}

	milestones := map[string]*gitea.Milestone{}
	for _, milestone := range giteaMilestones {
		milestones[milestone.Title] = milestone
	}
	return milestones, nil
}

// giteaMilestones returns a map of all gitea labels.
func (m *migrator) giteaLabels(giteaOwner string, giteaRepo string) (map[string]*gitea.Label, error) {
	opt := gitea.ListLabelsOptions{}
	giteaLabels, _, err := m.gitea.ListRepoLabels(giteaOwner, giteaRepo, opt)
	if err != nil {
		return nil, err
	}

	labels := map[string]*gitea.Label{}
	for _, label := range giteaLabels {
		labels[label.Name] = label
	}
	return labels, nil
}

// giteaMilestones returns a map of all gitea issues.
func (m *migrator) giteaIssues(giteaOwner string, giteaRepo string) (map[string]*gitea.Issue, error) {
	issues := map[string]*gitea.Issue{}
	for page := 1; ; page++ {
		opt := gitea.ListIssueOption{
			ListOptions: gitea.ListOptions{
				Page: page,
			},
			State: "all",
		}
		giteaIssues, _, err := m.gitea.ListRepoIssues(giteaOwner, giteaRepo, opt)
		if err != nil {
			return nil, err
		}
		if len(giteaIssues) == 0 {
			return issues, nil
		}

		for _, issue := range giteaIssues {
			issues[issue.Title] = issue
		}
	}
}
