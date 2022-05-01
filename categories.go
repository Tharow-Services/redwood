package main

// storage and loading of categories

import (
	_ "embed"
	"fmt"
	"github.com/andybalholm/redwood/efs"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/andybalholm/dhash"
)

// A weight contains the point values assigned to a rule+category combination.
type weight struct {
	points    int // points per occurrence
	maxPoints int // maximum points per page
}

// An action is the action assigned to a category.
type action int

const (
	BLOCK  action = -1
	IGNORE action = 0
	ALLOW  action = 1
	ACL    action = 2
)

func (a action) String() string {
	switch a {
	case BLOCK:
		return "block"
	case IGNORE:
		return "ignore"
	case ALLOW:
		return "allow"
	case ACL:
		return "acl"
	}
	return "<invalid action>"
}

// A category represents one of the categories of filtering rules.
type category struct {
	name        string          // the directory name
	description string          // the name presented to users
	action      action          // the action to be taken with a page in this category
	weights     map[rule]weight // the weight for each rule
	invisible   bool            // use invisible GIF instead of block page
}

// isEmbedded are the categories being loaded from efs.embedded
var isEmbedded = false

// LoadCategories loads the category configuration files
func (conf *config) LoadCategories(dirName string) error {
	if conf.Categories == nil {
		conf.Categories = map[string]*category{}
	}
	isEmbedded = efs.IsEmbed(dirName)
	return conf.loadCategories(dirName, nil)
}

func (conf *config) loadCategories(dirName string, parent *category) error {
	if isEmbedded {
		dirName = efs.ToEmbed(dirName)
	}
	info, err := efs.ReadDir(dirName)
	if err != nil {
		return fmt.Errorf("could not read category directory: %v", err)
	}
	var notM = !isEmbedded || stringSet(conf.BuiltInCategories).contains("?ALL?")
	for _, fi := range info {
		var t = notM || stringSet(conf.BuiltInCategories).contains(fi.Name())
		if name := fi.Name(); fi.IsDir() && name[0] != '.' && t {
			categoryPath := efs.Join(dirName, name)
			c, err := loadCategory(categoryPath, parent)
			if err != nil {
				log.Printf("Error loading category %s: %v", name, err)
				continue
			}
			conf.Categories[c.name] = c

			// Load child categories.
			err = conf.loadCategories(categoryPath, c)
			if err != nil {
				log.Printf("Error loading child categories of %s: %v", c.name, err)
			}
		}
	}

	return nil
}

// loadCategory loads the configuration for one category
func loadCategory(dirname string, parent *category) (c *category, err error) {
	if isEmbedded {
		dirname = efs.ToEmbed(dirname)
	}
	c = new(category)
	c.weights = make(map[rule]weight)
	c.name = efs.Base(dirname)
	if parent != nil {
		c.name = parent.name + "/" + c.name
	}
	c.description = c.name

	confFile := efs.Join(dirname, "category.yml")
	conf, err := efs.ConfigFile(confFile)
	if err != nil {
		return nil, err
	}
	s, _ := conf.Get("description")
	if s != "" {
		c.description = s
	}

	s, _ = conf.Get("action")
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "allow":
		c.action = ALLOW
	case "ignore":
		c.action = IGNORE
	case "block":
		c.action = BLOCK
	case "acl":
		c.action = ACL
	case "":
		// No-op.
	default:
		return nil, fmt.Errorf("unrecognized action %s in %s", s, confFile)
	}

	s, _ = conf.Get("invisible")
	if s != "" {
		c.invisible, err = strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			log.Printf("Invalid setting for 'invisible' in %s: %q", confFile, s)
		}
	}

	if parent != nil {
		// Copy rules from parent category.
		for r, w := range parent.weights {
			c.weights[r] = w
		}
	}

	ruleFiles, err := efs.Glob(efs.Join(dirname, "*.list"))
	if err != nil {
		return nil, fmt.Errorf("error listing rule files: %v", err)
	}
	sort.Strings(ruleFiles)
	for _, list := range ruleFiles {
		r, err := efs.Open(list)
		if err != nil {
			log.Println(err)
			continue
		}
		cr := newConfigReader(r)

		defaultWeight := 0

		for {
			line, err := cr.ReadLine()
			if err != nil {
				break
			}

			r, line, err := parseRule(line)
			if err != nil {
				log.Printf("Error in line %d of %s: %s", cr.LineNo, list, err)
				continue
			}

			var w weight
			n, _ := fmt.Sscan(line, &w.points, &w.maxPoints)
			if n == 0 {
				w.points = defaultWeight
			}

			if r.t == defaultRule {
				defaultWeight = w.points
			} else {
				c.weights[r] = w
			}
		}
	}
	log.Printf("We Have Loaded Category: %s", c.description)
	return c, nil
}

// collectRules collects the rules from all the categories and adds
// them to URLRules and phraseRules.
func (conf *config) collectRules() {
	for _, c := range conf.Categories {
		for rule := range c.weights {
			switch rule.t {
			case contentPhrase:
				conf.ContentPhraseList.addPhrase(rule.content)
			case imageHash:
				content := rule.content
				threshold := -1
				if dash := strings.Index(content, "-"); dash != -1 {
					t, err := strconv.Atoi(content[dash+1:])
					if err != nil {
						log.Printf("%v: %v", rule, err)
						continue
					}
					threshold = t
					content = content[:dash]
				}
				h, err := dhash.Parse(content)
				if err != nil {
					log.Printf("%v: %v", rule, err)
					continue
				}
				conf.ImageHashes = append(conf.ImageHashes, dhashWithThreshold{h, threshold})
			default:
				conf.URLRules.AddRule(rule)
			}
		}
	}
	conf.ContentPhraseList.findFallbackNodes(0, nil)
	conf.URLRules.finalize()
}

// score returns c's score for a page that matched
// the rules in tally. The keys are the rule names, and the values
// are the counts of how many times each rule was matched.
func (c *category) score(tally map[rule]int, conf *config) int {
	total := 0
	weights := c.weights
	for r, count := range tally {
		w := weights[r]
		if conf.CountOnce {
			total += w.points
			continue
		}
		p := w.points * count
		if w.maxPoints != 0 && (p > 0 && p > w.maxPoints || p < 0 && p < w.maxPoints) {
			p = w.maxPoints
		}
		total += p
	}
	return total
}

// categoryScores returns a map containing a page's score for each category.
func (conf *config) categoryScores(tally map[rule]int) map[string]int {
	if len(tally) == 0 {
		return nil
	}

	scores := make(map[string]int)
	for _, c := range conf.Categories {
		s := c.score(tally, conf)
		if s != 0 {
			scores[c.name] = s
		}
	}
	return scores
}
