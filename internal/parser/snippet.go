package parser

import (
	"regexp"
	"sort"
	"strings"

	tf "github.com/galeone/tensorflow/tensorflow/go"
	tg "github.com/galeone/tfgo"
)

type Snippet struct {
	attributes  map[string]string
	content     string
	description string // preceeding paragraph
	language    string
}

func (s Snippet) Cmds() []string {
	var cmds []string

	firstHasDollar := false
	lines := strings.Split(s.content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "$") {
			firstHasDollar = true
			line = strings.TrimLeft(line, "$")
		} else if firstHasDollar {
			// If the first line was prefixed with "$",
			// then all commands should be as well.
			// If they are not, it's likely that
			// they indicate the expected output instead.
			continue
		}

		line = strings.TrimSpace(line)

		if line == "" {
			continue
		}

		cmds = append(cmds, line)
	}

	return cmds
}

func (s Snippet) FirstCmd() string {
	cmds := s.Cmds()
	if len(cmds) > 0 {
		return cmds[0]
	}
	return ""
}

var descriptionEndingsRe = regexp.MustCompile(`[:?!]$`)

func (s Snippet) Description() string {
	result := descriptionEndingsRe.ReplaceAllString(s.description, ".")
	return result
}

func (s Snippet) Name() string {
	return s.attributes["name"]
}

func (s Snippet) Language() string {
	return s.language
}

type Snippets []Snippet

func (s Snippets) Lookup(name string) (Snippet, bool) {
	for _, snippet := range s {
		if snippet.Name() == name {
			return snippet, true
		}
	}
	return Snippet{}, false
}

func (s Snippets) Names() (result []string) {
	for _, snippet := range s {
		result = append(result, snippet.Name())
	}
	return result
}

type tuple struct {
	index int
	score float32
}

func (s Snippets) PredictLangs() Snippets {
	model := tg.LoadModel("data/model", []string{"serve"}, nil)
	var xs []string

	for _, snippet := range s {
		joined := strings.Join(snippet.Cmds(), "\n")
		xs = append(xs, joined)
	}

	txs, err := tf.NewTensor(xs)
	if err != nil {
		panic(err)
	}

	// Model's details:
	// signature_def['serving_default']:
	// 	The given SavedModel SignatureDef contains the following input(s):
	// 		inputs['inputs'] tensor_info:
	// 			dtype: DT_STRING
	// 			shape: (-1)
	// 			name: Placeholder:0
	// 	The given SavedModel SignatureDef contains the following output(s):
	// 		outputs['classes'] tensor_info:
	// 			dtype: DT_STRING
	// 			shape: (-1, 54)
	// 			name: head/Tile:0
	// 		outputs['scores'] tensor_info:
	// 			dtype: DT_FLOAT
	// 			shape: (-1, 54)
	// 			name: head/predictions/probabilities:0
	// 	Method name is: tensorflow/serving/classify

	// requires $ curl -LO https://storage.googleapis.com/tensorflow/libtensorflow/libtensorflow-cpu-darwin-x86_64-2.5.0.tar.gz
	// in $ cat libtensorflow-cpu-darwin-x86_64-2.5.0.tar.gz | tar xz --directory /usr/local
	result := model.Exec([]tf.Output{
		model.Op("head/Tile", 0),
		model.Op("head/predictions/probabilities", 0),
	}, map[tf.Output]*tf.Tensor{
		model.Op("Placeholder", 0): txs,
	})

	// shape is symetric
	langItems := (result[0].Value()).([][]string)
	scoreItems := (result[1].Value()).([][]float32)

	for i, elem := range scoreItems {
		sorted := make([]tuple, len(elem))

		for ii := range elem {
			sorted[ii] = tuple{index: ii, score: scoreItems[i][ii]}
		}

		sort.Slice(sorted, func(a int, b int) bool {
			return sorted[a].score > sorted[b].score
		})

		s[i].language = langItems[i][sorted[0].index]

		// print top5
		// for ii := 0; ii < 5; ii++ {
		// 	s := sorted[ii]
		// 	fmt.Printf("%d.) %s: %f\n", i, langItems[i][s.index], sorted[ii].score)
		// }
	}

	return s
}
