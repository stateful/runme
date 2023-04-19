package cmd

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"gonum.org/v1/gonum/mat"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mgutz/ansi"
	"github.com/mitchellh/go-wordwrap"
	"github.com/pkg/errors"
	"github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/stateful/runme/internal/tween"
)

const MODE_SELECT = 0
const MODE_ERROR = 1
const MODE_BRANCH = 2
const MODE_BRANCH_ENTER = 3

const ANIM_TYPE_SPIN = 0
const ANIM_TYPE_SWIRL = 1

type model struct {
	// choices  []string         // items on the to-do list
	// cursor   int              // which to-do list item our cursor is pointing at
	// selected map[int]struct{} // which to-do items are selected

	tick int

	width  int
	height int

	animStart float64
	animType  int

	mode int

	selectCursor int

	errorResponse string
	errorFinished bool

	branchCursor  int
	branchOptions []string

	branchBuffer string
}

func assistCmd() *cobra.Command {
	cmd := cobra.Command{
		Use:     "assist",
		Aliases: []string{"assistant"},
		Hidden:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := tea.NewProgram(NewModel())
			if _, err := p.Run(); err != nil {
				return err
			}

			return nil
		},
	}

	cmd.AddCommand(environmentDumpCmd())

	setDefaultFlags(&cmd)

	return &cmd
}

const animDuration = 1.0

// var animEaseFunc = CubicBezier(.82, .07, .45, .85)
var animEaseFunc = tween.CubicBezier(.7, .2, .38, .88)

func NewModel() model {
	return model{
		animStart: -1,
	}
}

func (m model) AnimTime() *float64 {
	if m.animStart < 0 {
		return nil
	}

	time := m.ElapsedTime() - m.animStart

	return &time
}

func (m model) ElapsedTime() float64 {
	return float64(m.tick) / 60.0
}

func (m *model) StartAnim(animType int) {
	if m.AnimTime() == nil {
		m.animType = animType
		m.animStart = m.ElapsedTime()
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		// case " ":
		// 	return m, func() tea.Msg { return AnimStartMsg{} }

		case "down", "j":
			switch m.mode {
			case MODE_SELECT:
				if m.selectCursor < 1 {
					m.selectCursor++
					m.StartAnim(ANIM_TYPE_SWIRL)
				}
			case MODE_BRANCH:
				if m.branchCursor < len(m.branchOptions)-1 {
					m.branchCursor++
				}
			}

		case "up", "k":
			switch m.mode {
			case MODE_SELECT:
				if m.selectCursor > 0 {
					m.selectCursor--
					m.StartAnim(ANIM_TYPE_SWIRL)
				}
			case MODE_BRANCH:
				if m.branchCursor > 0 {
					m.branchCursor--
				}
			}

		case "enter":
			switch m.mode {
			case MODE_SELECT:
				switch m.selectCursor {
				case 0:
					m.branchBuffer = ""
					m.mode = MODE_BRANCH_ENTER
				case 1:
					m.mode = MODE_ERROR
					m.StartAnim(ANIM_TYPE_SPIN)
					return m, CreateDiagnosisStream
				}
			}
		}

	case DiagnosisStreamCreatedMsg:
		m.errorResponse = ""
		m.errorFinished = false
		return m, RecvDiagnosisStream(msg.stream)

	case DiagnosisProgressMsg:
		m.errorResponse += msg.delta
		return m, RecvDiagnosisStream(msg.stream)

	case DiagnosisFinishedMsg:
		m.errorFinished = true

	// case AnimStartMsg:
	// 	m.StartAnim()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case TickMsg:
		m.tick++

		animTime := m.AnimTime()
		if animTime != nil && *animTime >= animDuration {
			m.animStart = -1
		}

		return m, Tick
	}

	return m, nil
}

type DiagnosisFinishedMsg struct{}
type DiagnosisProgressMsg struct {
	delta  string
	stream *openai.ChatCompletionStream
}
type DiagnosisStreamCreatedMsg struct {
	stream *openai.ChatCompletionStream
}

func RecvDiagnosisStream(stream *openai.ChatCompletionStream) tea.Cmd {
	return func() tea.Msg {
		response, err := stream.Recv()

		if errors.Is(err, io.EOF) {
			return DiagnosisFinishedMsg{}
		}

		if err != nil {
			panic(err)
		}

		return DiagnosisProgressMsg{
			delta:  response.Choices[0].Delta.Content,
			stream: stream,
		}
	}
}

func CreateDiagnosisStream() tea.Msg {
	errorMsg, err := clipboard.ReadAll()
	if err != nil {
		panic(err)
	}

	stream, err := diagnoseError(errorMsg)
	if err != nil {
		panic(err)
	}

	return DiagnosisStreamCreatedMsg{
		stream: stream,
	}
}

func (m *model) RunmeButton(rad int, cr float64) [][]string {
	fratio := 2

	width := rad * 2
	height := rad * 2

	scaled_width := width * fratio

	matr := make([][]string, height)
	for i := range matr {
		matr[i] = make([]string, scaled_width)
		for j := range matr[i] {
			if i == 0 && j == 0 {
				matr[i][j] = "╭"
			} else if i == 0 && j == scaled_width-1 {
				matr[i][j] = "╮"
			} else if i == height-1 && j == scaled_width-1 {
				matr[i][j] = "╯"
			} else if i == height-1 && j == 0 {
				matr[i][j] = "╰"
			} else if i == 0 || i == height-1 {
				// matr[i][j] = "-"
				matr[i][j] = "─"
			} else if j == 0 || j == scaled_width-1 {
				// matr[i][j] = "|"
				matr[i][j] = "│"
			} else {
				matr[i][j] = " "
			}
		}
	}

	// cr := 8.5

	viewportTransform := func(p []float64) {
		p[0] *= float64(fratio)
		p[0] += float64(scaled_width) / 2
		p[1] += float64(height) / 2
	}

	{
		viewportTransform := func(p []float64) {
			viewportTransform(p)
			p[0] -= cr / 3
		}

		t := 0.

		animTime := m.AnimTime()
		if animTime != nil {
			t = *animTime / animDuration
		}

		t = float64(animEaseFunc(0, 1, t))

		pts := make([][]float64, 3)
		for i := range pts {
			th := float64(i) * ((2 * math.Pi) / 3)

			switch m.animType {
			case ANIM_TYPE_SPIN:
				th += t * 2 * math.Pi

				pts[i] = []float64{
					math.Cos(th) * cr,
					math.Sin(th) * cr,
				}
			case ANIM_TYPE_SWIRL:
				pts[i] = []float64{
					math.Cos(th) * cr,
					math.Sin(th) * cr,
				}

				fac := pts[i][0]
				pts[i][0] += fac * (math.Cos(t*2*math.Pi) - 1)
			}

			// transform
			viewportTransform(pts[i])
		}

		drawLine(matr, pts[0], pts[1])
		drawLine(matr, pts[1], pts[2])
		drawLine(matr, pts[2], pts[0])
	}

	return matr
}

func (m *model) ContentView() (s string) {
	if m.mode == MODE_SELECT {
		opts := []string{
			"Suggest Branch Name",
			"Diagnose Error",
		}

		for i, opt := range opts {
			cursor := " "

			if i == m.selectCursor {
				cursor = ">"
			}

			s += cursor
			s += " "

			name := opt

			if i == m.selectCursor {
				name = ansi.Color(name, "white+b")
			}

			s += name
			s += "\n\n"
		}
	}

	if m.mode == MODE_ERROR {
		s += m.errorResponse

		if m.errorFinished {
			s += "\n\n"
			s += "\x1B[7m * \x1B[0m Finished, press any key to return to the menu."
		}
	}

	return
}

func (m model) View() (s string) {
	if m.width <= 0 {
		return ""
	}

	rad := 12
	cr := 7.5

	t := m.ElapsedTime()

	display := make([][]string, 2*rad+4)

	for i := range display {
		display[i] = make([]string, m.width)

		for j := range display[i] {
			display[i][j] = " "
		}
	}

	playButton := m.RunmeButton(rad, cr)

	for i := range playButton {
		for j := range playButton[i] {
			floatFac := int(math.Round((math.Sin(2*t) + 1) / 2))

			display[i+1+floatFac][j+2] = playButton[i][j]
		}
	}

	marginLeft := 4 * rad
	paddingHor := 6

	contentView := m.ContentView()
	contentView = wordwrap.WrapString(contentView, uint(m.width-marginLeft-2*paddingHor))

	contentLines := strings.Split(contentView, "\n")

	sliceStart := len(contentLines) - (2*rad - 4)
	if sliceStart < 0 {
		sliceStart = 0
	}

	for i, line := range contentLines[sliceStart:] {
		for j, c := range line {
			display[i+3][j+marginLeft+paddingHor] = string(c)
		}
	}

	for i := range display {
		for _, c := range display[i] {
			s += c
		}
		s += "\n"
	}

	return

	// fratio := 2

	// rad := 15
	// width := rad * 2
	// height := rad * 2

	// scaled_width := width * fratio

	// matr := make([][]string, height)
	// for i := range matr {
	// 	matr[i] = make([]string, scaled_width)
	// 	for j := range matr[i] {
	// 		if i == 0 || i == height-1 {
	// 			matr[i][j] = "-"
	// 		} else if j == 0 || j == scaled_width-1 {
	// 			matr[i][j] = "|"
	// 		} else {
	// 			matr[i][j] = " "
	// 		}
	// 	}
	// }

	// cr := 8.5

	// viewportTransform := func(p []float64) {
	// 	p[0] *= float64(fratio)
	// 	p[0] += float64(scaled_width) / 2
	// 	p[1] += float64(height) / 2
	// }

	// {
	// 	viewportTransform := func(p []float64) {
	// 		viewportTransform(p)
	// 		p[0] -= cr / 3
	// 	}

	// 	t := 0.

	// 	animTime := m.AnimTime()
	// 	if animTime != nil {
	// 		t = *animTime / animDuration
	// 	}

	// 	t = float64(animEaseFunc(0, 1, t))

	// 	pts := make([][]float64, 3)
	// 	for i := range pts {

	// 		th := float64(i) * ((2 * math.Pi) / 3)
	// 		th += t * 2 * math.Pi

	// 		pts[i] = []float64{
	// 			math.Cos(th) * cr,
	// 			math.Sin(th) * cr,
	// 		}

	// 		// transform
	// 		viewportTransform(pts[i])
	// 	}

	// 	drawLine(matr, pts[0], pts[1])
	// 	drawLine(matr, pts[1], pts[2])
	// 	drawLine(matr, pts[2], pts[0])
	// }

	// return
}

type TickMsg struct{}

func Tick() tea.Msg {
	time.Sleep(time.Second / 60.0)
	return TickMsg{}
}

func (m model) Init() tea.Cmd {
	return Tick
}

func diagnoseError(e string) (*openai.ChatCompletionStream, error) {
	authToken, ok := os.LookupEnv("OPENAI_TOKEN")
	if !ok {
		return nil, errors.Errorf("No auth token provided")
	}

	client := openai.NewClient(authToken)

	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				// Unsupported node version 16.7.2, Required >=18
				Content: fmt.Sprintf("How can I solve the following error?\n\n%v", e),
				// Content: "What is the circumference of the Earth?",
			},
		},
	}

	stream, err := client.CreateChatCompletionStream(
		context.Background(),
		req,
	)

	if err != nil {
		return nil, err
	}

	return stream, nil

	// defer stream.Close()

	// fmt.Printf("Stream response: ")
	// for {
	// 	response, err := stream.Recv()
	// 	if errors.Is(err, io.EOF) {
	// 		fmt.Println("\nStream finished")
	// 		return
	// 	}

	// 	if err != nil {
	// 		fmt.Printf("\nStream error: %v\n", err)
	// 		return
	// 	}

	// 	fmt.Printf(response.Choices[0].Delta.Content)
	// }
}

// func main() {
// }

func matPrint(X mat.Matrix) {
	fa := mat.Formatted(X, mat.Prefix(""), mat.Squeeze())
	fmt.Printf("%v\n", fa)
}

func drawLine(grid [][]string, pt1 []float64, pt2 []float64) {
	diff := diff2d(pt2, pt1)
	ang := math.Atan2(diff[1], diff[0])

	c := lineChar(ang)

	samples := 200
	for t := 0; t <= samples; t++ {
		tf := (float64(t) / float64(samples))

		p := lerp2d(pt1, pt2, tf)

		setGrid(grid, p, c, 0.5)
	}
}

func lineChar(ang float64) string {
	// ⟍ ╲ ─ │ ╱ ╲ \ /

	deg := ang * (180 / math.Pi)
	thresh := 10.0

	if deg < 0 {
		deg += 180
	}

	if math.Abs(deg-0) <= thresh || math.Abs(deg-360) <= thresh || math.Abs(deg-180) <= thresh {
		// return "─"
		return "-"
	} else if math.Abs(deg-90) <= thresh || math.Abs(deg-270) <= thresh {
		// return "│"
		return "|"
	} else if (deg <= 90) || (deg >= 180 && deg <= 270) {
		// return "╲"
		return "\\"
	} else {
		// return "╱"
		return "/"
	}

	return " "
}

type PointTransformFunc func([]float64)

func drawCircle(grid [][]string, center []float64, rad float64, transformPt PointTransformFunc) {
	samples := 400
	for t := 0; t <= samples; t++ {
		tf := (float64(t) / float64(samples)) * 2 * math.Pi

		p := []float64{
			math.Cos(tf) * rad,
			math.Sin(tf) * rad,
		}

		transformPt(p)

		setGrid(grid, p, "#", 0.5)
	}
}

func setGrid(grid [][]string, p []float64, char string, tol float64) {
	px := int(math.Round(p[0]))
	py := int(math.Round(p[1]))

	dist2 := (p[0]-float64(px))*(p[0]-float64(px)) + (p[1]-float64(py))*(p[1]-float64(py))

	if px >= 0 && px < len(grid[0]) && py >= 0 && py < len(grid) && dist2 <= tol*tol {
		grid[py][px] = char
	}
}

func diff2d(p1 []float64, p2 []float64) []float64 {
	return []float64{
		p1[0] - p2[0],
		p1[1] - p2[1],
	}
}

func lerp2d(p1 []float64, p2 []float64, t float64) []float64 {
	return []float64{
		p1[0]*(1-t) + p2[0]*t,
		p1[1]*(1-t) + p2[1]*t,
	}
}
