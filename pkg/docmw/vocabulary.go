package docmw

import (
	"io/fs"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

type Vocabulary struct {
	Topics   map[string]struct{}
	DocTypes map[string]struct{}
}

type vocabularyYAML struct {
	Topics   []string `yaml:"topics"`
	DocTypes []string `yaml:"doc_types"`
}

func LoadVocabularyFS(fsys fs.FS, filePath string) (*Vocabulary, error) {
	if fsys == nil {
		return nil, errors.New("fs is required")
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("vocabulary path is required")
	}
	raw, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return nil, errors.Wrapf(err, "read vocabulary %q", filePath)
	}

	var payload vocabularyYAML
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return nil, errors.Wrap(err, "decode vocabulary yaml")
	}

	v := &Vocabulary{
		Topics:   map[string]struct{}{},
		DocTypes: map[string]struct{}{},
	}
	for _, topic := range payload.Topics {
		normalized := strings.TrimSpace(strings.ToLower(topic))
		if normalized == "" {
			continue
		}
		v.Topics[normalized] = struct{}{}
	}
	for _, docType := range payload.DocTypes {
		normalized := strings.TrimSpace(strings.ToLower(docType))
		if normalized == "" {
			continue
		}
		v.DocTypes[normalized] = struct{}{}
	}
	return v, nil
}

func (v *Vocabulary) ValidateDoc(doc ModuleDoc, strict bool) error {
	if v == nil {
		return nil
	}

	docType := strings.TrimSpace(strings.ToLower(doc.DocType))
	if strict && docType != "" {
		if _, ok := v.DocTypes[docType]; !ok {
			return errors.Errorf("unknown doc_type %q", doc.DocType)
		}
	}
	if !strict {
		return nil
	}
	for _, topic := range doc.Topics {
		normalized := strings.TrimSpace(strings.ToLower(topic))
		if normalized == "" {
			continue
		}
		if _, ok := v.Topics[normalized]; !ok {
			return errors.Errorf("unknown topic %q", topic)
		}
	}
	return nil
}
