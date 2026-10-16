// Generated from server/form_rules.json.
part of 'main.dart';
const Map<String,dynamic> formRules = {
  "name": {
    "max": 200,
    "unit": "characters",
    "zh": "填写企业全称或产品名称",
    "en": "Company or product name",
    "sample_zh": "河北冀农农机具有限公司 / 液压翻转犁1LF-440",
    "sample_en": "Company name / Hydraulic plough 1LF-440",
    "required": [
      "vendor",
      "product"
    ]
  },
  "name_en": {
    "max": 200,
    "unit": "characters",
    "zh": "英文名称选填",
    "en": "English name is optional",
    "sample_zh": "Hebei Jinong Machinery Co., Ltd.",
    "sample_en": "Hebei Jinong Machinery Co., Ltd.",
    "required": []
  },
  "region_name": {
    "max": 100,
    "unit": "characters",
    "zh": "填写省、市、区县，或选择已有地区",
    "en": "Province, city and district",
    "sample_zh": "河北省 邢台市 宁晋县",
    "sample_en": "Hebei, Xingtai, Ningjin",
    "required": [
      "vendor"
    ]
  },
  "main_products": {
    "max": 500,
    "unit": "characters",
    "zh": "说明主营设备或服务，用顿号分隔",
    "en": "Main products or services",
    "sample_zh": "翻转犁、旋耕机、农机配件",
    "sample_en": "Ploughs, tillers, machinery parts",
    "required": [
      "vendor"
    ]
  },
  "keywords": {
    "max": 500,
    "unit": "characters",
    "zh": "便于搜索，多个关键词用逗号分隔",
    "en": "Search keywords separated by commas",
    "sample_zh": "翻转犁,1LF-440,农机",
    "sample_en": "plough,1LF-440,machinery",
    "required": []
  },
  "phone": {
    "max": 32,
    "unit": "bytes",
    "zh": "联系电话或微信至少填写一项，支持手机和座机",
    "en": "Provide a phone number or WeChat; mobile or landline",
    "sample_zh": "13900000000 / 0319-1234567",
    "sample_en": "+86 13900000000",
    "required": []
  },
  "wechat": {
    "max": 100,
    "unit": "bytes",
    "zh": "填写可联系到您的微信号",
    "en": "Contact WeChat ID",
    "sample_zh": "jinong_service",
    "sample_en": "jinong_service",
    "required": []
  },
  "address": {
    "max": 500,
    "unit": "bytes",
    "zh": "填写实际企业地址，中文通常每字占3字节",
    "en": "Real company address; Chinese characters usually use 3 bytes",
    "sample_zh": "河北省邢台市宁晋县工业园区1号",
    "sample_en": "1 Industrial Park Road, Ningjin",
    "required": []
  },
  "description": {
    "max": 30000,
    "unit": "bytes",
    "zh": "介绍企业或产品特点，可分段填写",
    "en": "Company or product description",
    "sample_zh": "专业生产农机设备，提供选型和售后服务。",
    "sample_en": "Manufacturer of agricultural equipment.",
    "required": []
  },
  "description_en": {
    "max": 30000,
    "unit": "bytes",
    "zh": "英文介绍选填",
    "en": "Optional English description",
    "sample_zh": "We manufacture agricultural machinery.",
    "sample_en": "We manufacture agricultural machinery.",
    "required": []
  },
  "parameters": {
    "max": 10000,
    "unit": "bytes",
    "zh": "按型号、尺寸、功率等逐行填写",
    "en": "One specification per line",
    "sample_zh": "型号：1LF-440\n配套动力：120马力",
    "sample_en": "Model: 1LF-440\nPower: 120 hp",
    "required": []
  },
  "login_name": {
    "max": 100,
    "unit": "characters",
    "zh": "公司账号可使用企业简称，必须唯一",
    "en": "Unique company account; use a shorter company name",
    "sample_zh": "jinong / 河北冀农",
    "sample_en": "jinong",
    "required": []
  },
  "company_profile.short_name": {
    "max": 200,
    "unit": "characters",
    "zh": "企业展示简称",
    "en": "Brand name",
    "sample_zh": "冀农",
    "sample_en": "Jinong",
    "required": []
  },
  "company_profile.short_name_en": {
    "max": 200,
    "unit": "characters",
    "zh": "英文资料选填",
    "en": "Optional English information",
    "sample_zh": "Jinong",
    "sample_en": "Jinong",
    "required": []
  },
  "company_profile.summary": {
    "max": 1000,
    "unit": "characters",
    "zh": "简要介绍企业优势",
    "en": "Company highlights",
    "sample_zh": "以质量铸品牌，以信誉创辉煌",
    "sample_en": "Quality machinery, dependable service",
    "required": []
  },
  "company_profile.summary_en": {
    "max": 1000,
    "unit": "characters",
    "zh": "英文资料选填",
    "en": "Optional English information",
    "sample_zh": "Quality machinery, dependable service",
    "sample_en": "Quality machinery, dependable service",
    "required": []
  },
  "company_profile.established": {
    "max": 200,
    "unit": "characters",
    "zh": "成立年份或时间",
    "en": "Year established",
    "sample_zh": "1985",
    "sample_en": "1985",
    "required": []
  },
  "company_profile.staff": {
    "max": 200,
    "unit": "characters",
    "zh": "人员规模",
    "en": "Team size",
    "sample_zh": "100人",
    "sample_en": "100 employees",
    "required": []
  },
  "company_profile.factory_area": {
    "max": 200,
    "unit": "characters",
    "zh": "厂房面积及单位",
    "en": "Factory floor area and unit",
    "sample_zh": "10000平方米",
    "sample_en": "10000 m²",
    "required": []
  },
  "company_profile.equipment": {
    "max": 1000,
    "unit": "characters",
    "zh": "主要生产设备",
    "en": "Manufacturing equipment",
    "sample_zh": "数控车床、激光切割机",
    "sample_en": "CNC lathes, laser cutters",
    "required": []
  },
  "company_profile.equipment_en": {
    "max": 1000,
    "unit": "characters",
    "zh": "英文资料选填",
    "en": "Optional English information",
    "sample_zh": "CNC lathes, laser cutters",
    "sample_en": "CNC lathes, laser cutters",
    "required": []
  },
  "company_profile.capacity": {
    "max": 1000,
    "unit": "characters",
    "zh": "供货能力及时间单位",
    "en": "Supply capacity and time period",
    "sample_zh": "每月300台",
    "sample_en": "300 units per month",
    "required": []
  },
  "company_profile.capacity_en": {
    "max": 1000,
    "unit": "characters",
    "zh": "英文资料选填",
    "en": "Optional English information",
    "sample_zh": "300 units per month",
    "sample_en": "300 units per month",
    "required": []
  },
  "company_profile.website": {
    "max": 1000,
    "unit": "bytes",
    "zh": "须包含 http:// 或 https://",
    "en": "Include http:// or https://",
    "sample_zh": "https://www.example.com",
    "sample_en": "https://www.example.com",
    "required": []
  },
  "company_profile.contact_name": {
    "max": 200,
    "unit": "characters",
    "zh": "主要联系人姓名",
    "en": "Main contact person",
    "sample_zh": "张经理",
    "sample_en": "Mr. Zhang",
    "required": []
  },
  "company_profile.contact_name_en": {
    "max": 200,
    "unit": "characters",
    "zh": "英文资料选填",
    "en": "Optional English information",
    "sample_zh": "Mr. Zhang",
    "sample_en": "Mr. Zhang",
    "required": []
  },
  "company_profile.contacts.*.area": {
    "max": 500,
    "unit": "characters",
    "zh": "负责的省市或区域",
    "en": "Service area",
    "sample_zh": "河北、北京、天津",
    "sample_en": "Hebei, Beijing, Tianjin",
    "required": []
  },
  "company_profile.contacts.*.area_en": {
    "max": 500,
    "unit": "characters",
    "zh": "英文资料选填",
    "en": "Optional English information",
    "sample_zh": "Hebei, Beijing, Tianjin",
    "sample_en": "Hebei, Beijing, Tianjin",
    "required": []
  },
  "company_profile.contacts.*.name": {
    "max": 200,
    "unit": "characters",
    "zh": "区域业务联系人",
    "en": "Regional contact",
    "sample_zh": "李经理",
    "sample_en": "Ms. Li",
    "required": []
  },
  "company_profile.contacts.*.name_en": {
    "max": 200,
    "unit": "characters",
    "zh": "英文资料选填",
    "en": "Optional English information",
    "sample_zh": "Ms. Li",
    "sample_en": "Ms. Li",
    "required": []
  },
  "company_profile.contacts.*.phone": {
    "max": 32,
    "unit": "bytes",
    "zh": "此联系人电话或微信至少填一项",
    "en": "Provide phone or WeChat for this contact",
    "sample_zh": "0319-1234567",
    "sample_en": "+86 319 1234567",
    "required": []
  },
  "company_profile.contacts.*.wechat": {
    "max": 100,
    "unit": "bytes",
    "zh": "区域联系人的微信号",
    "en": "Regional WeChat ID",
    "sample_zh": "jinong_sales",
    "sample_en": "jinong_sales",
    "required": []
  },
  "company_profile.credentials.*.title": {
    "max": 100,
    "unit": "characters",
    "required": [],
    "zh": "图片标题，最多100字",
    "en": "Image title, up to 100 characters",
    "sample_zh": "生产车间／质量管理体系认证",
    "sample_en": "Production workshop / Quality certification"
  },
  "company_profile.credentials.*.title_en": {
    "max": 100,
    "unit": "characters",
    "required": [],
    "zh": "图片标题，最多100字",
    "en": "Image title, up to 100 characters",
    "sample_zh": "生产车间／质量管理体系认证",
    "sample_en": "Production workshop / Quality certification"
  },
  "company_profile.credentials.*.description": {
    "max": 500,
    "unit": "characters",
    "required": [],
    "zh": "图片说明，最多500字",
    "en": "Image description, up to 500 characters",
    "sample_zh": "说明图片展示的内容或认证情况",
    "sample_en": "Describe the scene or certification"
  },
  "company_profile.credentials.*.description_en": {
    "max": 500,
    "unit": "characters",
    "required": [],
    "zh": "图片说明，最多500字",
    "en": "Image description, up to 500 characters",
    "sample_zh": "说明图片展示的内容或认证情况",
    "sample_en": "Describe the scene or certification"
  },
  "company_profile.gallery_annotations.*.title": {
    "max": 100,
    "unit": "characters",
    "required": [],
    "zh": "图片标题，最多100字",
    "en": "Image title, up to 100 characters",
    "sample_zh": "生产车间／质量管理体系认证",
    "sample_en": "Production workshop / Quality certification"
  },
  "company_profile.gallery_annotations.*.title_en": {
    "max": 100,
    "unit": "characters",
    "required": [],
    "zh": "图片标题，最多100字",
    "en": "Image title, up to 100 characters",
    "sample_zh": "生产车间／质量管理体系认证",
    "sample_en": "Production workshop / Quality certification"
  },
  "company_profile.gallery_annotations.*.description": {
    "max": 500,
    "unit": "characters",
    "required": [],
    "zh": "图片说明，最多500字",
    "en": "Image description, up to 500 characters",
    "sample_zh": "说明图片展示的内容或认证情况",
    "sample_en": "Describe the scene or certification"
  },
  "company_profile.gallery_annotations.*.description_en": {
    "max": 500,
    "unit": "characters",
    "required": [],
    "zh": "图片说明，最多500字",
    "en": "Image description, up to 500 characters",
    "sample_zh": "说明图片展示的内容或认证情况",
    "sample_en": "Describe the scene or certification"
  }
};
