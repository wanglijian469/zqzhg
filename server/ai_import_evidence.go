package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"zqzt/exhibition/internal/bailian"
)

func aiProductRegionProblem(r AIRegion, total int, allowed map[int]bool) string {
	if !aiRegionValid(r, total) || !allowed[r.SourcePage] || r.ImageType == "logo" {
		return "ai_invalid_crop"
	}
	if (r.ImageType == "product_main" || r.ImageType == "product_photo") && (r.Box[2]-r.Box[0])*(r.Box[3]-r.Box[1]) > .85 {
		return "ai_whole_page_image"
	}
	return ""
}

func aiKeepSafeImages(x *AIProductExtraction, total int, scope []int) {
	allowed := map[int]bool{}
	for _, p := range scope {
		allowed[p] = true
	}
	images := []AIRegion{}
	for _, r := range x.Images {
		if problem := aiProductRegionProblem(r, total, allowed); problem != "" {
			x.ReviewIssues = append(x.ReviewIssues, fmt.Sprintf("图片需人工框选：PDF第%d页的%s建议范围%s无效或接近整页，已拒绝作为产品图片；请在来源页裁剪后重新关联。", r.SourcePage, r.ImageType, aiJSON(r.Box)))
		} else {
			images = append(images, r)
		}
	}
	x.Images = images
}

func (a *App) aiProductPage(ctx context.Context, task AIImportTask, stage, model, prompt string, parts []bailian.Part, scope []int) (AIProductExtraction, error) {
	var x AIProductExtraction
	if e := a.aiModel(ctx, task, stage, model, prompt, parts, &x); e != nil {
		return x, e
	}
	if problem := aiValidateExtraction(x, task.Total, scope); problem != nil {
		a.DB.Model(&AIImportModelCall{}).Where("task_id=? AND stage IN ? AND response_json<>''", task.ID, []string{stage, stage + "-repair"}).Update("error", "ai_invalid_evidence")
		repair := append(append([]bailian.Part{}, parts...), bailian.Part{Type: "text", Text: "The prior extraction failed evidence validation: " + problem.Error() + ". Allowed physical PDF pages are exactly " + aiJSON(scope) + ". Re-read ONLY these supplied original pages and return the requested schema. Crop a product photo tightly around the actual machine, excluding logos, headings, tables and footer; never return a near-whole brochure page as a product photo. Omit a crop you cannot locate and flag it for human review. Do not invent technical values. Previous extraction: " + aiJSON(x)})
		if e := a.aiModel(ctx, task, stage+"-evidence-repair", model, prompt, repair, &x); e != nil {
			return x, e
		}
		if e := aiValidateExtraction(x, task.Total, scope); e != nil {
			if e.Error() != "ai_invalid_crop" && e.Error() != "ai_whole_page_image" {
				return x, e
			}
			// Bad optional images must not prevent all later products being read.
			// Keep their raw proposal in the audit, flag manual cropping, never use
			// an unsafe image or fabricate a replacement.
			aiKeepSafeImages(&x, task.Total, scope)
			a.DB.Model(&AIImportModelCall{}).Where("task_id=? AND stage LIKE ? AND response_json<>''", task.ID, stage+"-evidence-repair%").Update("error", "ai_invalid_evidence")
			if e = aiValidateExtraction(x, task.Total, scope); e != nil {
				return x, e
			}
		}
		x.ReviewIssues = append(x.ReviewIssues, "该页结果曾因来源或裁剪范围不符而重新提取，请对照原页核对型号、技术原值及图片归属。")
		fp := fmt.Sprintf("%x", sha256.Sum256([]byte(model+aiExtractionRules+prompt+aiJSON(parts))))
		if e := a.aiRememberModel(task, stage, fp, &x); e != nil {
			return x, e
		}
	}
	return x, nil
}
