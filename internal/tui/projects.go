package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/sadopc/trackr/internal/store"
)

var projectColors = []string{"#6C63FF", "#2EC4B6", "#FF6B6B", "#F39C12", "#2ECC71", "#E74C3C", "#9B59B6", "#3498DB"}
var projectCategories = []string{"work", "personal", "learning", "freelance", "other"}

type projectsModel struct {
	store  *store.Store
	width  int
	height int

	projects     []store.Project
	tasks        []store.Task
	cursor       int
	taskCursor   int
	showArchived bool
	viewingTasks bool // true = viewing tasks of selected project

	formActive bool
	form       *huh.Form
	formType   string // "project", "task", "edit_project"

	// Form field pointers (survive value copies)
	formName     *string
	formColor    *string
	formCategory *string
	formTags     *string

	editingID int64 // project ID being edited
}

func newProjectsModel(s *store.Store) projectsModel {
	name, color, cat, tags := "", projectColors[0], "", ""
	return projectsModel{
		store:        s,
		formName:     &name,
		formColor:    &color,
		formCategory: &cat,
		formTags:     &tags,
	}
}

func (p *projectsModel) setSize(w, h int) {
	p.width = w
	p.height = h
}

type projectsDataMsg struct {
	projects []store.Project
}

type tasksDataMsg struct {
	tasks []store.Task
}

func (p projectsModel) refresh() tea.Cmd {
	return func() tea.Msg {
		projects, _ := p.store.ListProjects(p.showArchived)
		return projectsDataMsg{projects: projects}
	}
}

func (p projectsModel) refreshTasks() tea.Cmd {
	if p.cursor >= len(p.projects) {
		return nil
	}
	pid := p.projects[p.cursor].ID
	return func() tea.Msg {
		tasks, _ := p.store.ListTasks(pid, false)
		return tasksDataMsg{tasks: tasks}
	}
}

func (p projectsModel) update(msg tea.Msg) (projectsModel, tea.Cmd) {
	if p.formActive && p.form != nil {
		return p.updateForm(msg)
	}

	switch msg := msg.(type) {
	case projectsDataMsg:
		p.projects = msg.projects
		if p.cursor >= len(p.projects) {
			p.cursor = max(0, len(p.projects)-1)
		}
		return p, nil

	case tasksDataMsg:
		p.tasks = msg.tasks
		if p.taskCursor >= len(p.tasks) {
			p.taskCursor = max(0, len(p.tasks)-1)
		}
		return p, nil

	case tea.KeyMsg:
		if p.viewingTasks {
			return p.updateTaskView(msg)
		}
		return p.updateProjectList(msg)
	}
	return p, nil
}

func (p projectsModel) updateProjectList(msg tea.KeyMsg) (projectsModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		if p.cursor > 0 {
			p.cursor--
		}
	case key.Matches(msg, keys.Down):
		if p.cursor < len(p.projects)-1 {
			p.cursor++
		}
	case key.Matches(msg, keys.Enter):
		if len(p.projects) > 0 {
			p.viewingTasks = true
			p.taskCursor = 0
			return p, p.refreshTasks()
		}
	case key.Matches(msg, keys.New):
		return p.showNewProjectForm()
	case key.Matches(msg, keys.Delete):
		if len(p.projects) > 0 {
			proj := p.projects[p.cursor]
			p.store.ArchiveProject(proj.ID)
			return p, p.refresh()
		}
	case key.Matches(msg, keys.Export):
		if len(p.projects) > 0 {
			return p.showEditProjectForm()
		}
	}
	return p, nil
}

func (p projectsModel) updateTaskView(msg tea.KeyMsg) (projectsModel, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Back):
		p.viewingTasks = false
		return p, nil
	case key.Matches(msg, keys.Up):
		if p.taskCursor > 0 {
			p.taskCursor--
		}
	case key.Matches(msg, keys.Down):
		if p.taskCursor < len(p.tasks)-1 {
			p.taskCursor++
		}
	case key.Matches(msg, keys.New):
		return p.showNewTaskForm()
	case key.Matches(msg, keys.Delete):
		if len(p.tasks) > 0 {
			task := p.tasks[p.taskCursor]
			p.store.ArchiveTask(task.ID)
			return p, p.refreshTasks()
		}
	}
	return p, nil
}

func (p projectsModel) showNewProjectForm() (projectsModel, tea.Cmd) {
	*p.formName = ""
	*p.formColor = projectColors[0]
	*p.formCategory = "work"
	p.formType = "project"

	colorOptions := make([]huh.Option[string], len(projectColors))
	for i, c := range projectColors {
		colorOptions[i] = huh.NewOption(fmt.Sprintf("● %s", c), c)
	}
	catOptions := make([]huh.Option[string], len(projectCategories))
	for i, c := range projectCategories {
		catOptions[i] = huh.NewOption(c, c)
	}

	p.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Project Name").Value(p.formName),
			huh.NewSelect[string]().Title("Color").Options(colorOptions...).Value(p.formColor),
			huh.NewSelect[string]().Title("Category").Options(catOptions...).Value(p.formCategory),
		),
	).WithShowHelp(true).WithShowErrors(true)

	p.formActive = true
	return p, p.form.Init()
}

func (p projectsModel) showEditProjectForm() (projectsModel, tea.Cmd) {
	proj := p.projects[p.cursor]
	*p.formName = proj.Name
	*p.formColor = proj.Color
	*p.formCategory = proj.Category
	p.formType = "edit_project"
	p.editingID = proj.ID

	colorOptions := make([]huh.Option[string], len(projectColors))
	for i, c := range projectColors {
		colorOptions[i] = huh.NewOption(fmt.Sprintf("● %s", c), c)
	}
	catOptions := make([]huh.Option[string], len(projectCategories))
	for i, c := range projectCategories {
		catOptions[i] = huh.NewOption(c, c)
	}

	p.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Project Name").Value(p.formName),
			huh.NewSelect[string]().Title("Color").Options(colorOptions...).Value(p.formColor),
			huh.NewSelect[string]().Title("Category").Options(catOptions...).Value(p.formCategory),
		),
	).WithShowHelp(true).WithShowErrors(true)

	p.formActive = true
	return p, p.form.Init()
}

func (p projectsModel) showNewTaskForm() (projectsModel, tea.Cmd) {
	*p.formName = ""
	*p.formTags = ""
	p.formType = "task"

	p.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Task Name").Value(p.formName),
			huh.NewInput().Title("Tags (comma-separated)").Value(p.formTags),
		),
	).WithShowHelp(true).WithShowErrors(true)

	p.formActive = true
	return p, p.form.Init()
}

func (p projectsModel) updateForm(msg tea.Msg) (projectsModel, tea.Cmd) {
	// Check for escape to cancel form
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "esc" {
			p.formActive = false
			p.form = nil
			return p, nil
		}
	}

	form, cmd := p.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		p.form = f
	}

	if p.form.State == huh.StateCompleted {
		p.formActive = false
		switch p.formType {
		case "project":
			if *p.formName != "" {
				p.store.CreateProject(*p.formName, *p.formColor, *p.formCategory)
			}
			return p, p.refresh()
		case "edit_project":
			if *p.formName != "" {
				p.store.UpdateProject(p.editingID, *p.formName, *p.formColor, *p.formCategory)
			}
			return p, p.refresh()
		case "task":
			if *p.formName != "" && p.cursor < len(p.projects) {
				p.store.CreateTask(p.projects[p.cursor].ID, *p.formName, *p.formTags)
			}
			return p, p.refreshTasks()
		}
	}

	return p, cmd
}

func (p projectsModel) view() string {
	if p.formActive && p.form != nil {
		title := titleStyle.Render("New Project")
		if p.formType == "edit_project" {
			title = titleStyle.Render("Edit Project")
		} else if p.formType == "task" {
			title = titleStyle.Render("New Task")
		}
		formView := p.form.View()
		content := lipgloss.JoinVertical(lipgloss.Left, title, "", formView)
		return panelStyle.Width(p.width - 4).Render(content)
	}

	if p.viewingTasks {
		return p.renderTaskView()
	}
	return p.renderProjectList()
}

func (p projectsModel) renderProjectList() string {
	w := p.width - 4
	title := titleStyle.Render("Projects")

	if len(p.projects) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			mutedStyle.Render("No projects yet. Press n to create one."),
		)
		return panelStyle.Width(w).Render(content)
	}

	var rows []string
	rows = append(rows, title)
	rows = append(rows, "")

	// Table header
	header := mutedStyle.Render(fmt.Sprintf("  %-3s %-24s %-12s %-12s", "", "Name", "Category", "Color"))
	rows = append(rows, header)

	for i, proj := range p.projects {
		colorDot := lipgloss.NewStyle().Foreground(lipgloss.Color(proj.Color)).Render("●")
		cursor := "  "
		style := normalItemStyle
		if i == p.cursor {
			cursor = "> "
			style = selectedItemStyle
		}
		row := style.Render(fmt.Sprintf("%s%s %-24s %-12s", cursor, colorDot, proj.Name, proj.Category))
		rows = append(rows, row)
	}

	rows = append(rows, "")
	rows = append(rows, mutedStyle.Render("  n: new  e: edit  d: archive  enter: tasks  esc: back"))

	return panelStyle.Width(w).Render(strings.Join(rows, "\n"))
}

func (p projectsModel) renderTaskView() string {
	w := p.width - 4
	proj := p.projects[p.cursor]
	colorDot := lipgloss.NewStyle().Foreground(lipgloss.Color(proj.Color)).Render("●")
	title := titleStyle.Render(fmt.Sprintf("%s %s — Tasks", colorDot, proj.Name))

	if len(p.tasks) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left,
			title,
			"",
			mutedStyle.Render("No tasks. Press n to add one."),
		)
		return panelStyle.Width(w).Render(content)
	}

	var rows []string
	rows = append(rows, title)
	rows = append(rows, "")

	for i, task := range p.tasks {
		cursor := "  "
		style := normalItemStyle
		if i == p.taskCursor {
			cursor = "> "
			style = selectedItemStyle
		}
		tags := ""
		if task.Tags != "" {
			tags = mutedStyle.Render(" [" + task.Tags + "]")
		}
		rows = append(rows, style.Render(fmt.Sprintf("%s%s", cursor, task.Name))+tags)
	}

	rows = append(rows, "")
	rows = append(rows, mutedStyle.Render("  n: new task  d: archive  esc: back"))

	return panelStyle.Width(w).Render(strings.Join(rows, "\n"))
}
