package main

import "testing"

func TestAIExactTechnicalLexemesAndSourceWhitelist(t *testing.T) {
	var x AIProductExtraction
	err := aiDecode(`{"product_name":"X","source_pages":"1","models":[{"model_name":1234567890123456789,"parameters":[{"parameter_name":"压力","raw_value":1.2300e2,"raw_unit":"MPa","source_page":"1","review_required":true}]}]}`, &x)
	if err != nil {
		t.Fatal(err)
	}
	if x.Models[0].ModelName != "1234567890123456789" || x.Models[0].Parameters[0].RawValue != "1.2300e2" || x.SourcePages[0] != 1 || len(x.ReviewIssues) == 0 {
		t.Fatal("technical literal was rounded or conversion not flagged", x)
	}
	if aiDecode(`{"models":[{"model_name":"X","parameters":[{"raw_value":{"guess":"12"}}]}]}`, &x) == nil {
		t.Fatal("compound technical value accepted")
	}
	company := AICompanyExtraction{Data: Payload{Name: "有来源的企业", Phone: "unproven", CompanyProfile: &CompanyProfile{Website: "unproven"}}, FieldSources: map[string][]int{"name": {1}, "phone": {9}}}
	p := aiSourcedCompany(company, 2)
	if p.Name != "有来源的企业" || p.Phone != "" || (p.CompanyProfile != nil && p.CompanyProfile.Website != "") {
		t.Fatal("unsourced company facts accepted", p)
	}
	p = aiReviewedPayload(Payload{ProductDetails: &ProductDetails{Models: []AIProductModel{{ModelName: "X", Parameters: []AIParameter{{RawValue: "10~12", ReviewRequired: true}}}}}})
	if p.ProductDetails.Models[0].Parameters[0].ReviewRequired || p.ProductDetails.Models[0].Parameters[0].RawValue != "10~12" {
		t.Fatal("review changed technical value")
	}
}
