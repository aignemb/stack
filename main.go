package main
// TODO need to stop q from acting as quit during typing

import (
	"fmt"
	"os"
	"runtime/debug"
	"encoding/json"
	"time"
	"strconv"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

//types
type Priority int
type View string
type File string

type task struct {
	ID string
	Title string
	Priority Priority
}

type Input struct {
	titleField textinput.Model
	priorityField textinput.Model
}

type Prefix struct {
	value string
	wasSet bool
}

type Flags struct {
	n bool // new task created
}

type model struct {
	f Flags
	view View //current view
	lastView View
	cursor map[View]int // can complicate with vstack, hstack and change the key type for the map later
	prefix Prefix
	inputs Input
	stack []task
	heap []task
	archive []task
	msgs []string
}

//constants
const (
	stackFile File = "data/stack.json"
	heapFile File = "data/heap.json"
	archiveFile File = "data/archive.json"

	High Priority = 1
	Medium Priority = 2
	Low Priority = 3

	stackView View = "stack"
	heapView View = "heap"
	archiveView View = "archive"
	teditView View = "edit task"
)

// funcs

func getRoot() string {
	path, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(path)
}

func ensureData() error {
	err := os.MkdirAll(filepath.Join(getRoot(), "data"), 0755)
	if err != nil {
		return err
	}

	files := []File{stackFile, heapFile, archiveFile}
	for _, f := range files {
		if _, err := os.Stat(string(f)); os.IsNotExist(err) {
			err := os.WriteFile(string(f), []byte("[]"), 0644)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func loadModel() (model) {
	m := model{
		f: Flags{false},
		view: stackView,
		lastView: "",
		cursor: map[View]int{
			stackView: 0,
			heapView: 0,
			archiveView: 0,
		},
		prefix: Prefix{"", false},
		inputs: Input{titleField: textinput.New(), priorityField: textinput.New()},
		stack: []task{}, // fetch stack
		heap: []task{},//fetch heap
		archive: []task{},//fetch archive
		msgs: []string{},
	}

	root := getRoot()
	//m.msgs = append(m.msgs, filepath.Join(root, string(stackFile)))
	data, err := os.ReadFile(filepath.Join(root, string(stackFile)))
	if err != nil {
		//statusMsg = fmt.Sprintf("Error reading stack, please review stackFile %s\nError: %v", stackFile, err)
		fmt.Printf("Error reading stack, please review stackFile %s\nError: %v", stackFile, err)
	}
	json.Unmarshal(data, &m.stack)

	data, err = os.ReadFile(filepath.Join(root, string(heapFile)))
	if err != nil {
		//statusMsg = fmt.Sprintf("Error reading heap, please review heapFile %s\nError: %v", heapFile, err)
		fmt.Printf("Error reading heap, please review heapFile %s\nError: %v", heapFile, err)
	}
	json.Unmarshal(data, &m.heap)

	data, err = os.ReadFile(filepath.Join(root, string(archiveFile)))
	if err != nil {
		//statusMsg = fmt.Sprintf("Error reading archive, please review archiveFile %s\nError: %v", archiveFile, err)
		fmt.Printf("Error reading archive, please review archiveFile %s\nError: %v", archiveFile, err)
	}
	json.Unmarshal(data, &m.archive)

	m.inputs.titleField.Placeholder = "Task Description"
	m.inputs.titleField.SetWidth(50)

	m.inputs.priorityField.Placeholder = "(1-3)"
	m.inputs.priorityField.SetWidth(10)

	return m
}

func (m model) store() []string {
	root := getRoot()
	var msgs []string

	data, err := json.MarshalIndent(m.stack, "", "  ")
	if err != nil {
		//statusMsg = fmt.Sprintf("Error storing stack\nError: %v", err)
		msgs = append(msgs, fmt.Sprintf("Error marshalling stack\nError: %v", err))
	}
	err = os.WriteFile(filepath.Join(root, string(stackFile)), data, 0644)
	if err != nil {
		msgs = append(msgs, fmt.Sprintf("Error storing stack\nError: %v", err))
	}

	data, err = json.MarshalIndent(m.heap, "", "  ")
	if err != nil {
		//statusMsg = fmt.Sprintf("Error storing heap\nError: %v", err)
		msgs = append(msgs, fmt.Sprintf("Error storing heap\nError: %v", err))
	}
	err = os.WriteFile(filepath.Join(root, string(heapFile)), data, 0644)
	if err != nil {
		msgs = append(msgs, fmt.Sprintf("Error storing heap\nError: %v", err))
	}

	data, err = json.MarshalIndent(m.archive, "", "  ")
	if err != nil {
		//statusMsg = fmt.Sprintf("Error storing archive\nError: %v", err)
		fmt.Printf("Error storing archive\nError: %v", err)
	}
	err = os.WriteFile(filepath.Join(root, string(archiveFile)), data, 0644)
	if err != nil {
		msgs = append(msgs, fmt.Sprintf("Error storing archive\nError: %v", err))
	}
	return msgs
}

func (t task) display() string {
	return fmt.Sprintf(" [%d] %s", t.Priority, t.Title)
}

func assert(condition bool, msg string, msgStack []string) {
    if !condition {
		mash := msg + "\n\n"
		mash += "----------\nMessage Stack\n"
		for _, m := range msgStack {
			mash += fmt.Sprintf("%s\n", m)
		}
		mash += "----------\n\n"
        panic(fmt.Sprintf(mash))
    }
}

func insert(l []task, i int, t task) []task {
	if len(l) == 0 {
		l = append(l, t)
	} else {
		l = append(l[:i+1], l[i:]...)
		l[i] = t
	}
	return l
}

func edit(l []task, i int, t task) []task {
	l[i].Title = t.Title
	l[i].Priority = t.Priority
	return l
}

func remove(l []task, i int) []task {
	l = append(l[:i], l[i+1:]...)
	return l
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if !m.prefix.wasSet {
			m.prefix.value = ""
		}
		m.prefix.wasSet = false

		if m.view == teditView {
			switch msg.String() {
			case "ctrl+c":
				m.msgs = append(m.msgs, m.store()...)
				return m, tea.Quit
			case "tab":
				if m.inputs.titleField.Focused() {
					m.inputs.titleField.Blur()
					return m, m.inputs.priorityField.Focus()
				} else {
					m.inputs.priorityField.Blur()
					return m, m.inputs.titleField.Focus()
				}
			case "esc":
				m.inputs.titleField.Blur()
				m.inputs.priorityField.Blur()
				m.inputs.titleField.SetValue("")
				m.inputs.priorityField.SetValue("")
				m.view = m.lastView
				m.lastView = ""
				return m, nil
			case "enter":
				m.inputs.titleField.Blur()
				m.inputs.priorityField.Blur()
				m.view = m.lastView
				m.lastView = ""

				var t task

				if m.f.n {
					now := time.Now()
					t.ID = fmt.Sprintf("%d%02d%02d%02d%02d%02d",
					now.Year(),
					now.Month(),
					now.Day(),
					now.Hour(), 
					now.Minute(),
					now.Second(),)
				}
				n, err := strconv.Atoi(m.inputs.priorityField.Value())
				if err != nil {
					n = 2
					m.msgs = append(m.msgs, "invalid priority value, defaulting to 2")
				} else if n < 1 || n > 3 {
					n = 2
					m.msgs = append(m.msgs, "invalid priority value, defaulting to 2")
				}

				t.Title = m.inputs.titleField.Value()
				t.Priority = Priority(n)

				if m.f.n {
					switch m.view {
					case stackView:
						m.stack = insert(m.stack, m.cursor[stackView], t)
					case heapView:
						m.heap = insert(m.heap, m.cursor[heapView], t)
					case archiveView:
						m.archive = insert(m.archive, m.cursor[archiveView], t)
					}
				} else {
					switch m.view {
					case stackView:
						m.stack = edit(m.stack, m.cursor[stackView], t)
					case heapView:
						m.heap = edit(m.heap, m.cursor[heapView], t)
					case archiveView:
						m.archive = edit(m.archive, m.cursor[archiveView], t)
					}
				}
			m.msgs = append(m.msgs, m.store()...)
		}

		var taskCmd, priorityCmd tea.Cmd
		m.inputs.titleField, taskCmd = m.inputs.titleField.Update(msg)
		m.inputs.priorityField, priorityCmd = m.inputs.priorityField.Update(msg)
		return m, tea.Batch(taskCmd, priorityCmd)

	} else {

		switch msg.String() {
		case "ctrl+c","q":
			m.msgs = append(m.msgs, m.store()...)
			return m, tea.Quit

		case "d":
			if m.prefix.value == "d" {
				switch m.view {
				case stackView:
					m.stack = remove(m.stack, m.cursor[stackView])
					if m.cursor[stackView] >= len(m.stack) && len(m.stack) > 0{
						m.cursor[stackView] --
					}
				case heapView:
					m.heap = remove(m.heap, m.cursor[heapView])
					if m.cursor[heapView] >= len(m.heap) && len(m.heap) > 0{
						m.cursor[heapView] --
					}
				case archiveView:
					m.archive = remove(m.archive, m.cursor[archiveView])
					if m.cursor[archiveView] >= len(m.archive) && len(m.archive) > 0{
						m.cursor[archiveView] --
					}
				}
				m.msgs = append(m.msgs, m.store()...)
			} else {
				m.prefix.value = "d"
				m.prefix.wasSet = true
			}

		case "s":
			m.view = stackView

		case "h":
			m.view = heapView

		case "a":
			m.view = archiveView

		case "n":
			m.f.n = true
			if m.view == stackView || m.view == heapView {
				m.inputs.titleField.SetValue("")
				m.inputs.priorityField.SetValue("")
				m.lastView = m.view
				m.view = teditView
				return m, m.inputs.titleField.Focus()
			}

		case "e":
			m.f.n = false
			switch m.view {
			case stackView:
				m.inputs.titleField.SetValue(m.stack[m.cursor[stackView]].Title)
				m.inputs.priorityField.SetValue(strconv.Itoa(int(m.stack[m.cursor[stackView]].Priority)))

			case heapView:
				m.inputs.titleField.SetValue(m.heap[m.cursor[heapView]].Title)
				m.inputs.priorityField.SetValue(strconv.Itoa(int(m.heap[m.cursor[heapView]].Priority)))

			case archiveView:
				m.inputs.titleField.SetValue(m.archive[m.cursor[archiveView]].Title)
				m.inputs.priorityField.SetValue(strconv.Itoa(int(m.archive[m.cursor[archiveView]].Priority)))
			}
			m.lastView = m.view
			m.view = teditView
			return m, m.inputs.titleField.Focus()

		case "j":
			switch m.view {
			case stackView:
				if m.cursor[stackView] < len(m.stack) - 1 {
					m.cursor[stackView] ++
				}
			case heapView:
				if m.cursor[heapView] < len(m.heap) - 1 {
					m.cursor[heapView] ++
				}
			case archiveView:
				if m.cursor[archiveView] < len(m.archive) - 1 {
					m.cursor[archiveView] ++
				}
			}

		case "J":
			switch m.view {
			case stackView:
				if m.cursor[stackView] < len(m.stack) - 1 {
					m.stack[m.cursor[stackView]], m.stack[m.cursor[stackView] + 1] = 
					m.stack[m.cursor[stackView] + 1], m.stack[m.cursor[stackView]] 
					m.cursor[stackView] ++
				}
			case heapView:
				if m.cursor[heapView] < len(m.heap) - 1 {
					m.heap[m.cursor[heapView]], m.heap[m.cursor[heapView] + 1] = 
					m.heap[m.cursor[heapView] + 1], m.heap[m.cursor[heapView]] 
					m.cursor[heapView] ++
				}
			case archiveView:
				if m.cursor[archiveView] < len(m.archive) - 1 {
					m.archive[m.cursor[archiveView]], m.archive[m.cursor[archiveView] + 1] = 
					m.archive[m.cursor[archiveView] + 1], m.archive[m.cursor[archiveView]] 
					m.cursor[archiveView] ++
				}
			}
			m.msgs = append(m.msgs, m.store()...)

		case "k":
			switch m.view {
			case stackView:
				if m.cursor[stackView] > 0 {
					m.cursor[stackView] --
				}
			case heapView:
				if m.cursor[heapView] > 0 {
					m.cursor[heapView] --
				}
			case archiveView:
				if m.cursor[archiveView] > 0 {
					m.cursor[archiveView] --
				}
			}

		case "K":
			switch m.view {
			case stackView:
				if m.cursor[stackView] > 0 {
					m.stack[m.cursor[stackView]], m.stack[m.cursor[stackView] - 1] = 
					m.stack[m.cursor[stackView] - 1], m.stack[m.cursor[stackView]] 
					m.cursor[stackView] --
				}
			case heapView:
				if m.cursor[heapView] > 0 {
					m.heap[m.cursor[heapView]], m.heap[m.cursor[heapView] - 1] = 
					m.heap[m.cursor[heapView] - 1], m.heap[m.cursor[heapView]] 
					m.cursor[heapView] --
				}
			case archiveView:
				if m.cursor[archiveView] > 0 {
					m.archive[m.cursor[archiveView]], m.archive[m.cursor[archiveView] - 1] = 
					m.archive[m.cursor[archiveView] - 1], m.archive[m.cursor[archiveView]] 
					m.cursor[archiveView] --
				}
			}
			m.msgs = append(m.msgs, m.store()...)

		case "p":
			if m.view == stackView {
				m.heap = insert(m.heap, 0, m.stack[m.cursor[stackView]])
				m.stack = remove(m.stack, m.cursor[stackView])
				if m.cursor[stackView] >= len(m.stack) && len(m.stack) > 0{
					m.cursor[stackView] --
				}
			} else if m.view == heapView{
				m.stack = insert(m.stack, 0, m.heap[m.cursor[heapView]])
				m.heap = remove(m.heap, m.cursor[heapView])
				if m.cursor[heapView] >= len(m.heap) && len(m.heap) > 0{
					m.cursor[heapView] --
				}
			}
			m.msgs = append(m.msgs, m.store()...)
		case "z":
			if m.view == stackView {
				m.archive = insert(m.archive, 0, m.stack[m.cursor[stackView]])
				m.stack = remove(m.stack, m.cursor[stackView])
				if m.cursor[stackView] >= len(m.stack) && len(m.stack) > 0{
					m.cursor[stackView] --
				}
			} else if m.view == heapView {
				m.archive = insert(m.archive, 0, m.heap[m.cursor[heapView]])
				m.heap = remove(m.heap, m.cursor[heapView])
				if m.cursor[heapView] >= len(m.heap) && len(m.heap) > 0{
					m.cursor[heapView] --
				}
			} else if m.view == archiveView {
				m.heap = insert(m.heap, 0, m.archive[m.cursor[archiveView]])
				m.archive = remove(m.archive, m.cursor[archiveView])
				if m.cursor[archiveView] >= len(m.archive) && len(m.archive) > 0{
					m.cursor[archiveView] --
				}
			}
			m.msgs = append(m.msgs, m.store()...)
		}
	}
}
return m, nil
}

func (m model) View() tea.View{

	assert(m.cursor[stackView] >= 0 && m.cursor[stackView] < len(m.stack) || m.cursor[stackView] == 0 && len(m.stack) == 0, 
	fmt.Sprintf("stack cursor out of bounds\ncursor: %d\nlength of stack: %d", m.cursor[stackView], len(m.stack)), m.msgs)
	assert(m.cursor[heapView] >= 0 && m.cursor[heapView] < len(m.heap) || m.cursor[heapView] == 0 && len(m.heap) == 0, 
	fmt.Sprintf("heap cursor out of bounds\ncursor: %d\nlength of heap: %d", m.cursor[heapView], len(m.heap)), m.msgs)
	assert(m.cursor[archiveView] >= 0 && m.cursor[archiveView] < len(m.archive) || m.cursor[archiveView] == 0 && len(m.archive) == 0, 
	fmt.Sprintf("archive cursor out of bounds\ncursor: %d\nlength of archive: %d", m.cursor[archiveView], len(m.archive)), m.msgs)

	var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#0000FF"))

	var stackStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFD700"))

	var topStackStyle = stackStyle.Bold(true)

	var archiveStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7F7F7F"))

	var previewStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#7F7F7F"))

	s := titleStyle.Render(fmt.Sprintf("%s", m.view)) + "\n"

	switch m.view {
	case stackView:
		if len(m.stack) == 0 {
			s += previewStyle.Render("(empty stack)")
		} else {
			for i, t := range m.stack {
				if m.cursor[stackView] == i {
					s += ">"
				} else {
					s += " "
				}
				if i == 0 {
					s += topStackStyle.Render(t.display()) + "\n"
				} else {
					s += stackStyle.Render(t.display()) + "\n"
				}
			}
		}
	case heapView:
		if len(m.heap) == 0 {
			s += previewStyle.Render("(empty heap)")
		} else {
			for i, t := range m.heap {
				if m.cursor[heapView] == i {
					s += ">"
				} else {
					s += " "
				}
				s += t.display() + "\n"
			}
		}
	case archiveView:
		if len(m.archive) == 0 {
			s += previewStyle.Render("(empty archive)")
		} else {
			for i, t := range m.archive {
				if m.cursor[archiveView] == i {
					s += ">"
				} else {
					s += " "
				}
				s += archiveStyle.Render(t.display()) + "\n"
			}
		}
	case teditView:
		s += fmt.Sprintf("task: %s\n\npriority: %s\n", m.inputs.titleField.View(), m.inputs.priorityField.View())
	}

	//s += fmt.Sprintf("\n----------\nCursor Details\nStack: %d\nHeap: %d\nArchive: %d\n----------\n", m.cursor[stackView], m.cursor[heapView], m.cursor[archiveView])

	s += "\n"
	for i, msg := range m.msgs {
		s += fmt.Sprintf("%d. %s\n", i + 1,  msg)
	}
	v := tea.NewView(s)
	v.AltScreen = true
	return v
}

func main() {
	if err := ensureData(); err != nil {
		fmt.Printf("failed to initialize data files: %v", err)
	}
	p := tea.NewProgram(loadModel())

    defer func() {
        if r := recover(); r != nil {
            p.ReleaseTerminal()
            fmt.Printf("panic: %+v\n", r)
        }
    }()

    if _, err := p.Run(); err != nil {
        p.ReleaseTerminal()
        fmt.Printf("there has been an error: %+v", err)
		debug.PrintStack()
    }
}

