package ui

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
)

// Spinner creates and starts a spinner with the given text
func Spinner(text string) (*pterm.SpinnerPrinter, error) {
	spinner, err := pterm.DefaultSpinner.Start(text)
	if err != nil {
		return nil, err
	}
	return spinner, nil
}

// Success displays a success message with a checkmark
func Success(message string) {
	pterm.Success.Println(message)
}

// Error displays an error message
func Error(message string) {
	pterm.Error.Println(message)
}

// Warning displays a warning message
func Warning(message string) {
	pterm.Warning.Println(message)
}

// Info displays an info message
func Info(message string) {
	pterm.Info.Println(message)
}

// Println prints a blank line
func Println() {
	pterm.Println()
}

// Header displays a section header
func Header(text string) {
	pterm.DefaultHeader.WithFullWidth().WithBackgroundStyle(pterm.NewStyle(pterm.BgLightBlue)).WithMargin(10).Println(text)
}

// Section displays a section title
func Section(text string) {
	pterm.Println()
	pterm.DefaultSection.Println(text)
}

// Table creates and renders a table with headers and data
func Table(headers []string, data [][]string) error {
	tableData := pterm.TableData{headers}
	tableData = append(tableData, data...)
	return pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(tableData).Render()
}

// ProgressBar creates a new progress bar
func ProgressBar(title string, total int) (*pterm.ProgressbarPrinter, error) {
	pb, err := pterm.DefaultProgressbar.WithTotal(total).WithTitle(title).Start()
	if err != nil {
		return nil, err
	}
	return pb, nil
}

// Panel displays content in a styled panel
func Panel(title, content string) {
	panel := pterm.DefaultBox.WithTitle(title).WithTitleTopCenter().Sprint(content)
	pterm.Println(panel)
}

// BulletList displays a bulleted list
func BulletList(items []string) error {
	list := pterm.DefaultBulletList.WithItems(
		convertToBulletListItems(items),
	)
	return list.Render()
}

func convertToBulletListItems(items []string) []pterm.BulletListItem {
	bulletItems := make([]pterm.BulletListItem, len(items))
	for i, item := range items {
		bulletItems[i] = pterm.BulletListItem{Level: 0, Text: item}
	}
	return bulletItems
}

// StepList displays numbered steps with status
type Step struct {
	Name   string
	Status string // "pending", "running", "success", "error"
}

func StepList(steps []Step) {
	for i, step := range steps {
		prefix := fmt.Sprintf("%d.", i+1)
		switch step.Status {
		case "running":
			pterm.Printf("%s %s %s\n", pterm.LightBlue(prefix), pterm.Cyan("⟳"), step.Name)
		case "success":
			pterm.Printf("%s %s %s\n", pterm.LightBlue(prefix), pterm.Green("✓"), pterm.Gray(step.Name))
		case "error":
			pterm.Printf("%s %s %s\n", pterm.LightBlue(prefix), pterm.Red("✗"), step.Name)
		default: // pending
			pterm.Printf("%s %s %s\n", pterm.LightBlue(prefix), pterm.Gray("○"), pterm.Gray(step.Name))
		}
	}
}

// Confirmation asks user for yes/no confirmation
func Confirmation(message string) bool {
	result, _ := pterm.DefaultInteractiveConfirm.Show(message)
	return result
}

// KeyValue displays a key-value pair in a styled format
func KeyValue(key, value string) {
	pterm.Printf("%s %s\n", pterm.LightCyan(key+":"), value)
}

// MultiSpinner manages multiple spinners for parallel tasks
type MultiSpinner struct {
	spinners map[string]*pterm.SpinnerPrinter
}

func NewMultiSpinner() *MultiSpinner {
	return &MultiSpinner{
		spinners: make(map[string]*pterm.SpinnerPrinter),
	}
}

func (ms *MultiSpinner) Add(name, text string) error {
	spinner, err := pterm.DefaultSpinner.Start(text)
	if err != nil {
		return err
	}
	ms.spinners[name] = spinner
	return nil
}

func (ms *MultiSpinner) Update(name, text string) {
	if spinner, ok := ms.spinners[name]; ok {
		spinner.UpdateText(text)
	}
}

func (ms *MultiSpinner) Success(name, text string) {
	if spinner, ok := ms.spinners[name]; ok {
		spinner.Success(text)
		delete(ms.spinners, name)
	}
}

func (ms *MultiSpinner) Fail(name, text string) {
	if spinner, ok := ms.spinners[name]; ok {
		spinner.Fail(text)
		delete(ms.spinners, name)
	}
}

// PrintLogo displays the Kubeasy logo/banner
func PrintLogo() {
	logo := pterm.DefaultBigText.WithLetters(
		putils.LettersFromString("Kubeasy"),
	)
	_ = logo.Render()
	pterm.Println()
}

// WaitMessage displays a message while executing a function
func WaitMessage(message string, fn func() error) error {
	spinner, err := pterm.DefaultSpinner.Start(message)
	if err != nil {
		return err
	}

	err = fn()
	if err != nil {
		spinner.Fail(message)
		return err
	}

	spinner.Success(message)
	return nil
}

// TimedSpinner shows a spinner with elapsed time
func TimedSpinner(message string, fn func() error) error {
	start := time.Now()
	spinner, err := pterm.DefaultSpinner.Start(message)
	if err != nil {
		return err
	}

	// Update spinner text with elapsed time
	done := make(chan error)
	go func() {
		done <- fn()
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case err := <-done:
			elapsed := time.Since(start).Round(time.Second)
			if err != nil {
				spinner.Fail(fmt.Sprintf("%s (failed after %s)", message, elapsed))
				return err
			}
			spinner.Success(fmt.Sprintf("%s (completed in %s)", message, elapsed))
			return nil
		case <-ticker.C:
			elapsed := time.Since(start).Round(time.Second)
			spinner.UpdateText(fmt.Sprintf("%s (%s)", message, elapsed))
		}
	}
}

// ValidationResult displays validation results in a formatted way
func ValidationResult(name string, passed bool, details []string) {
	if passed {
		pterm.Success.Printf("%s: All checks passed\n", name)
	} else {
		pterm.Error.Printf("%s: Some checks failed\n", name)
	}

	if len(details) > 0 {
		for _, detail := range details {
			if passed {
				pterm.Printf("  %s %s\n", pterm.Green("✓"), detail)
			} else {
				pterm.Printf("  %s %s\n", pterm.Red("✗"), detail)
			}
		}
	}
}
