package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var errAuthAPISetupCancelled = errors.New("official API setup cancelled")

type authAPISetupStep int

const (
	authAPISetupIntro authAPISetupStep = iota
	authAPISetupTokenInput
)

type authAPISetupWizardModel struct {
	step      authAPISetupStep
	token     string
	input     textinput.Model
	message   string
	err       error
	cancelled bool
}

func newAuthAPISetupWizardModel() authAPISetupWizardModel {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = "ntn_..."
	input.EchoMode = textinput.EchoPassword
	input.EchoCharacter = '•'
	input.CharLimit = 512

	return authAPISetupWizardModel{
		step:  authAPISetupIntro,
		input: input,
	}
}

func runAuthAPISetupWizard() (string, error) {
	model := newAuthAPISetupWizardModel()
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return "", err
	}

	wizard, ok := finalModel.(authAPISetupWizardModel)
	if !ok {
		return "", fmt.Errorf("unexpected wizard model type %T", finalModel)
	}
	if wizard.cancelled {
		return "", errAuthAPISetupCancelled
	}
	if strings.TrimSpace(wizard.token) == "" {
		return "", fmt.Errorf("official API token is required")
	}

	return wizard.token, nil
}

func (m authAPISetupWizardModel) Init() tea.Cmd {
	return nil
}

func (m authAPISetupWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case authAPISetupIntro:
			switch msg.String() {
			case "ctrl+c", "q", "esc":
				m.cancelled = true
				return m, tea.Quit
			case "o":
				if err := openBrowserURL(apiSetupDocsURL); err != nil {
					m.err = err
				} else {
					m.message = "Opened integration docs in your browser."
					m.err = nil
				}
				return m, nil
			case "enter":
				m.step = authAPISetupTokenInput
				m.err = nil
				m.message = ""
				m.input.Focus()
				return m, nil
			}
		case authAPISetupTokenInput:
			switch msg.String() {
			case "ctrl+c", "q":
				m.cancelled = true
				return m, tea.Quit
			case "esc":
				m.step = authAPISetupIntro
				m.err = nil
				m.message = ""
				m.input.Blur()
				return m, nil
			case "enter":
				token := strings.TrimSpace(m.input.Value())
				if token == "" {
					m.err = fmt.Errorf("token cannot be empty")
					return m, nil
				}
				m.token = token
				return m, tea.Quit
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m authAPISetupWizardModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	var b strings.Builder
	b.WriteString(titleStyle.Render("Official Notion API Setup"))
	b.WriteString("\n\n")

	switch m.step {
	case authAPISetupIntro:
		b.WriteString("This setup stores a token for official Notion REST API features.\n")
		b.WriteString("Use an Internal integration token (not Public OAuth app credentials).\n")
		b.WriteString("Open: " + apiSetupInternalIntegrationsURL + "\n\n")
		b.WriteString("Enter: continue    o: open docs    q/esc: cancel\n")
	case authAPISetupTokenInput:
		b.WriteString("Paste your Notion integration token:\n")
		b.WriteString(m.input.View())
		b.WriteString("\n\n")
		b.WriteString("Expected: ntn_... (legacy secret_... also works)\n")
		b.WriteString("Enter: save    esc: back    q: cancel\n")
	}

	if m.message != "" {
		b.WriteString("\n")
		b.WriteString(infoStyle.Render(m.message))
	}
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(errStyle.Render("Error: " + m.err.Error()))
	}

	return b.String()
}
