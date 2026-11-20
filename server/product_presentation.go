package main

import (
	"encoding/json"
	"regexp"
	"strings"
)

var aiPageAnnotation = regexp.MustCompile(`(?i)[（(]\s*PDF\s*第\s*[0-9、,，~～\- ]+\s*页\s*[）)]`)
var aiSourceLine = regexp.MustCompile(`(?im)^\s*(?:\*\*|#{1,6}\s*)?(?:画册来源|材料来源|PDF来源|识别过程|整理说明)(?:\*\*)?\s*[:：]?[^\r\n]*$`)

// Presentation filtering is deliberately narrow. Never change a numeric value,
// an original drawing, or an ordinary product sentence containing the word PDF.
func aiCleanPresentation(s string) string {
	return strings.TrimSpace(aiSourceLine.ReplaceAllString(aiPageAnnotation.ReplaceAllString(s, ""), ""))
}
func standardAIProductPayload(p Payload) Payload {
	p.Description = aiCleanPresentation(p.Description)
	p.DescriptionEN = aiCleanPresentation(p.DescriptionEN)
	p.Parameters = aiCleanPresentation(p.Parameters)
	if p.ProductDetails != nil {
		var details ProductDetails
		b, _ := json.Marshal(p.ProductDetails)
		_ = json.Unmarshal(b, &details)
		p.ProductDetails = &details
		for i := range details.Features {
			details.Features[i] = aiCleanPresentation(details.Features[i])
		}
		for i := range details.Applications {
			details.Applications[i] = aiCleanPresentation(details.Applications[i])
		}
		for i := range details.Images {
			details.Images[i].Caption = aiCleanPresentation(details.Images[i].Caption)
		}
	}
	return p
}
