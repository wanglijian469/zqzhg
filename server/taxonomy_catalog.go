package main

import (
	"fmt"
	"gorm.io/gorm"
	"strings"
)

// Stable codes identify seed rows. Names, ordering, images and status remain editable.
const vendorCatalog = `农机整机|Agricultural machinery manufacturers
耕整地机械|Tillage machinery
播种施肥机械|Seeding and fertilizing machinery
收获机械|Harvesting machinery
田间管理与植保|Field management and crop protection
农机具及配套设备|Implements and supporting equipment
农机零部件|Agricultural machinery parts
动力与传动|Power and transmission
液压与底盘|Hydraulics and chassis
电气与智能控制|Electrical and intelligent control
灌溉与设施农业|Irrigation and protected agriculture
畜牧与饲料机械|Livestock and feed machinery
农产品加工机械|Agricultural processing machinery
机械加工与定制|Machining and custom manufacturing`

const productCatalog = `拖拉机与动力整机|Tractors and power units|轮式拖拉机:Wheeled tractors,履带拖拉机:Crawler tractors,手扶拖拉机:Walking tractors,其他农业动力整机:Other agricultural power units
耕整地机械|Tillage machinery|铧式犁:Mouldboard ploughs,圆盘犁:Disc ploughs,旋耕机:Rotary tillers,深松机:Subsoilers,耙地机械:Harrows,整地镇压机械:Land preparation and rollers
播种施肥机械|Seeding and fertilizing machinery|条播机:Seed drills,精量播种机:Precision planters,免耕播种机:No-till planters,育苗移栽机械:Seedling and transplanting machinery,施肥撒肥机械:Fertilizer spreaders,播种施肥联合作业机械:Combined seeding and fertilizing machinery
田间管理与植保机械|Field management and crop protection machinery|喷雾喷粉机械:Sprayers and dusters,农业植保无人机:Crop protection drones,中耕除草机械:Cultivators and weeders,修剪管理机械:Pruning and field management machinery
收获机械|Harvesting machinery|谷物收获机械:Grain harvesters,玉米收获机械:Maize harvesters,薯类根茎收获机械:Root and tuber harvesters,棉花收获机械:Cotton harvesters,饲草收获机械:Forage harvesters,果蔬收获机械:Fruit and vegetable harvesters
收获后处理与搬运设备|Post-harvest handling equipment|脱粒机械:Threshers,清选分级设备:Cleaning and grading equipment,烘干设备:Dryers,输送装卸设备:Conveying and loading equipment,农用搬运车辆:Agricultural transport vehicles
畜牧养殖与饲料机械|Livestock and feed machinery|饲料粉碎设备:Feed grinders,饲料混合制粒设备:Feed mixers and pellet mills,饲喂设备:Feeding equipment,挤奶设备:Milking equipment,养殖清粪设备:Manure removal equipment
农产品加工机械|Agricultural processing machinery|粮食加工设备:Grain processing equipment,油料加工设备:Oilseed processing equipment,果蔬加工设备:Fruit and vegetable processing equipment,初加工包装设备:Primary processing and packaging equipment
农机具及配套设备|Implements and supporting equipment|挂接与牵引装置:Hitches and towing devices,配重装置:Ballast equipment,农用挂车:Agricultural trailers,装载搬运属具:Loading and handling attachments,其他配套机具:Other supporting implements
农机易损件|Agricultural wear parts|耕作刀具与犁铧:Tillage blades and ploughshares,收割刀片与护刃器:Harvesting blades and guards,锤片与粉碎刀:Hammers and shredding blades,植保喷嘴与滤网:Spray nozzles and screens,摩擦片与衬片:Friction plates and linings
传动系统及零部件|Transmission systems and parts|齿轮与齿轮轴:Gears and gear shafts,变速箱与减速器:Gearboxes and reducers,传动轴与万向节:Drive shafts and universal joints,离合器与联轴器:Clutches and couplings,链条链轮:Chains and sprockets,皮带带轮:Belts and pulleys,动力输出装置:Power take-off units
发动机及配件|Engines and parts|柴油发动机:Diesel engines,汽油发动机:Petrol engines,缸体缸盖与曲柄连杆:Blocks heads and crank mechanisms,活塞缸套与配气机构:Pistons liners and valve mechanisms,燃油喷射系统:Fuel injection systems,进排气系统:Intake and exhaust systems,冷却润滑系统:Cooling and lubrication systems,滤清器:Filters
液压系统及零部件|Hydraulic systems and parts|液压油缸:Hydraulic cylinders,液压泵与马达:Hydraulic pumps and motors,液压阀:Hydraulic valves,液压管路与接头:Hydraulic hoses and fittings,液压过滤与油箱:Hydraulic filtration and tanks,液压总成:Hydraulic assemblies
行走底盘与转向制动|Running gear steering and brakes|轮胎轮辋:Tyres and rims,履带与支重轮:Tracks and rollers,车桥与驱动桥:Axles and drive axles,车架与悬挂:Frames and suspension,转向机构:Steering mechanisms,制动机构:Brake mechanisms
收获机械专用件|Harvesting machinery components|割台与拨禾机构:Headers and reels,脱粒滚筒与凹板:Threshing drums and concaves,分离清选机构:Separation and cleaning mechanisms,输送与升运机构:Conveyors and elevators,粮箱与卸粮机构:Grain tanks and unloading mechanisms,打捆机构:Baling mechanisms
耕作播种专用件|Tillage and seeding components|犁架与调幅翻转机构:Plough frames and reversing mechanisms,旋耕轴与刀座:Rotary shafts and blade holders,耙组与镇压机构:Harrow gangs and roller mechanisms,排种器与排肥器:Seed and fertilizer meters,开沟与覆土机构:Furrow openers and covering mechanisms,地轮与播深调节机构:Ground wheels and depth adjustment
轴承密封与标准件|Bearings seals and standard parts|滚动轴承:Rolling bearings,轴承座与滑动轴承:Bearing housings and plain bearings,油封与密封圈:Oil seals and sealing rings,垫片与组合密封:Gaskets and composite seals,螺栓螺母与垫圈:Bolts nuts and washers,销键与挡圈:Pins keys and retaining rings,弹簧:Springs
电气与智能农业设备|Electrical and smart agriculture equipment|起动充电与线束:Starting charging and wiring,照明与仪表:Lighting and instruments,传感器与控制器:Sensors and controllers,导航与自动驾驶:Navigation and automatic guidance,作业监测与终端:Operation monitoring and terminals,农业物联网设备:Agricultural IoT equipment
灌溉排水设备|Irrigation and drainage equipment|农用水泵:Agricultural water pumps,喷灌设备:Sprinkler irrigation equipment,滴灌微灌设备:Drip and micro irrigation,管材管件与阀门:Pipes fittings and valves,水肥一体化设备:Fertigation equipment,排水设备:Drainage equipment
设施农业设备及配件|Protected agriculture equipment and parts|温室骨架与覆盖材料:Greenhouse frames and coverings,通风降温设备:Ventilation and cooling equipment,保温加温设备:Insulation and heating equipment,补光设备:Supplemental lighting,育苗栽培设施:Seedling and cultivation facilities,设施环境控制设备:Greenhouse environment controls`

func seedAgricultureTaxonomy(tx *gorm.DB) error {
	// Reserve the two historical IDs on a fresh database, before generating new IDs.
	for _, c := range []Category{{ID: 1, HallID: 1, Name: "农业机械", NameEN: "Agricultural machinery", Enabled: true, CategoryKind: "legacy"}, {ID: 2, HallID: 1, Name: "农机配件", NameEN: "Machinery parts", Enabled: true, CategoryKind: "legacy"}} {
		if e := tx.FirstOrCreate(&c, Category{ID: c.ID}).Error; e != nil {
			return e
		}
	}
	for _, kind := range []string{"vendor", "product"} {
		catalog := vendorCatalog
		if kind == "product" {
			catalog = productCatalog
		}
		for i, line := range strings.Split(catalog, "\n") {
			parts := strings.Split(line, "|")
			code := fmt.Sprintf("agri.%s.%02d", kind, i+1)
			c := Category{HallID: 1, CategoryKind: kind, Code: &code, Name: parts[0], NameEN: parts[1], Enabled: true, SortOrder: (i + 1) * 10}
			if e := tx.Where("code=?", code).FirstOrCreate(&c).Error; e != nil {
				return e
			}
			if kind == "product" {
				for j, child := range strings.Split(parts[2], ",") {
					names := strings.SplitN(child, ":", 2)
					childCode := fmt.Sprintf("%s.%02d", code, j+1)
					sub := Category{HallID: 1, ParentID: c.ID, CategoryKind: kind, Code: &childCode, Name: names[0], NameEN: names[1], Enabled: true, SortOrder: (j + 1) * 10, SearchAliases: seedAliases(names[0])}
					if e := tx.Where("code=?", childCode).FirstOrCreate(&sub).Error; e != nil {
						return e
					}
				}
			}
		}
	}
	return nil
}
func seedAliases(name string) string {
	return map[string]string{"铧式犁": "翻转犁,液压翻转犁,犁", "耙地机械": "驱动耙,动力耙,圆盘耙", "齿轮与齿轮轴": "齿轮,齿轮轴", "耕作刀具与犁铧": "旋耕刀,犁铧,犁尖,犁壁", "收割刀片与护刃器": "收割刀,收割刀片,护刃器", "锤片与粉碎刀": "锤片,粉碎刀", "链条链轮": "链条,链轮", "皮带带轮": "三角带,皮带,带轮", "液压泵与马达": "液压泵,液压马达", "滚动轴承": "轴承", "油封与密封圈": "油封,密封圈,O形圈"}[name]
}
