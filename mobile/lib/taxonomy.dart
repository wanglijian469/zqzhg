part of 'main.dart';

String nativeCategoryPath(List<Json> categories, int id, AppModel m) {
  final parts = <String>[], seen = <int>{};
  while (id > 0 && seen.add(id)) {
    final item = categories.where((c) => c['id'] == id).firstOrNull;
    if (item == null) break;
    parts.insert(0, m.content(item, 'name'));
    id = (item['parent_id'] as num? ?? 0).toInt();
  }
  return parts.join(' / ');
}

const nativeBusinessNatures = <String, List<String>>{
  'manufacturer': ['生产制造', 'Manufacturer'],
  'dealer': ['经销代理', 'Dealer / agent'],
  'manufacturing_trading': ['工贸一体', 'Manufacturing and trading'],
  'processing': ['加工服务', 'Processing services'],
  'other': ['其他', 'Other'],
};

Future<int?> searchNativeCategories(
  BuildContext context,
  List<Json> cats,
  String kind,
  List<int> exclude,
) async {
  final m = ModelScope.of(context);
  var query = '';
  return Navigator.push<int>(
    context,
    MaterialPageRoute(
      builder: (screen) => StatefulBuilder(
        builder: (screen, change) => Scaffold(
          appBar: AppBar(
            title: Text(m.language == 'zh' ? '搜索选择分类' : 'Search categories'),
          ),
          body: SafeArea(
            child: Column(
              children: [
                Padding(
                  padding: const EdgeInsets.all(16),
                  child: TextField(
                    autofocus: true,
                    decoration: InputDecoration(
                      labelText: m.language == 'zh'
                          ? '分类名称或别名'
                          : 'Category name or alias',
                      prefixIcon: const Icon(Icons.search),
                    ),
                    onChanged: (v) => change(() => query = v.toLowerCase()),
                  ),
                ),
                Expanded(
                  child: ListView(
                    children: [
                      for (final cat in cats.where(
                        (c) =>
                            nativeCategoryPath(
                              cats,
                              c['id'],
                              m,
                            ).toLowerCase().contains(query) ||
                            (c['search_aliases'] ?? '')
                                .toString()
                                .toLowerCase()
                                .contains(query),
                      ))
                        ListTile(
                          minVerticalPadding: 12,
                          enabled:
                              cat['enabled'] == true &&
                              cat['selectable'] != false &&
                              !exclude.contains(cat['id']) &&
                              (kind != 'product' ||
                                  !cats.any(
                                    (c) => c['parent_id'] == cat['id'],
                                  )),
                          title: Text(nativeCategoryPath(cats, cat['id'], m)),
                          onTap: () => Navigator.pop(screen, cat['id']),
                        ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    ),
  );
}

class NativeTaxonomyFields extends StatefulWidget {
  final Json data;
  final String kind;
  final int hallID;
  final bool enabled;
  final ValueChanged<Json> onChanged;
  final Json issues;
  const NativeTaxonomyFields({
    super.key,
    required this.data,
    required this.kind,
    required this.hallID,
    required this.enabled,
    required this.onChanged,
    this.issues = const {},
  });
  @override
  State<NativeTaxonomyFields> createState() => _NativeTaxonomyFieldsState();
}

class _NativeTaxonomyFieldsState extends State<NativeTaxonomyFields> {
  final controllers = <String, TextEditingController>{};
  bool recommending = false;
  List<String> tokens(String value) => value
      .split(RegExp(r'[,，;；\n]'))
      .map((s) => s.trim())
      .where((s) => s.isNotEmpty)
      .toSet()
      .toList();
  String text(String key) => widget.data[key] is List
      ? (widget.data[key] as List).join(', ')
      : (widget.data[key] ?? '').toString();
  @override
  void initState() {
    super.initState();
    for (final key in [
      'brand',
      'model',
      'applicable_brands',
      'applicable_models',
    ]) {
      controllers[key] = TextEditingController(text: text(key));
    }
  }

  @override
  void didUpdateWidget(NativeTaxonomyFields old) {
    super.didUpdateWidget(old);
    for (final key in controllers.keys) {
      final controller = controllers[key]!;
      final equivalent = key.startsWith('applicable_')
          ? jsonEncode(tokens(controller.text)) ==
                jsonEncode(widget.data[key] ?? [])
          : controller.text == text(key);
      if (!equivalent) controller.text = text(key);
    }
  }

  @override
  void dispose() {
    for (final c in controllers.values) {
      c.dispose();
    }
    super.dispose();
  }

  void set(String key, dynamic value) => widget.onChanged({key: value});
  Future<void> recommend() async {
    final m = ModelScope.of(context);
    setState(() => recommending = true);
    try {
      final result = await m.api(
        '/public/category-recommendations',
        method: 'POST',
        data: {
          'hall_id': widget.hallID,
          'name': widget.data['name'] ?? '',
          'keywords': widget.data['keywords'] ?? '',
        },
      );
      if (!mounted) return;
      final items = rows(result['items']);
      if (items.isEmpty) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              m.language == 'zh'
                  ? '暂无匹配推荐，请搜索选择分类。'
                  : 'No matching recommendation. Search for a category.',
            ),
          ),
        );
        return;
      }
      final id = await showModalBottomSheet<int>(
        context: context,
        isScrollControlled: true,
        builder: (c) => SafeArea(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  m.language == 'zh'
                      ? '点击应用推荐分类'
                      : 'Select a recommended category',
                ),
                for (final item in items)
                  ListTile(
                    title: Text(
                      (m.language == 'zh' ? item['path'] : item['path_en']) ??
                          item['path'],
                    ),
                    onTap: () => Navigator.pop(c, item['id']),
                  ),
              ],
            ),
          ),
        ),
      );
      if (id != null && mounted && widget.enabled) {
        set('product_category_id', id);
      }
    } catch (e) {
      if (mounted) showError(context, e);
    } finally {
      if (mounted) setState(() => recommending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    final d = widget.data,
        secondary = (d['secondary_category_ids'] as List? ?? []).cast<int>();
    Widget picker(
      String field,
      int value,
      String label,
      ValueChanged<int> change,
      List<int> exclude,
    ) => Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: NativeCategoryPicker(
        kind: widget.kind,
        hallID: widget.hallID,
        value: value,
        label: label,
        excludeIDs: exclude,
        allowSuggest: field != 'secondary_category_ids',
        enabled: widget.enabled,
        issue: widget.issues[field] == null
            ? null
            : asMap(widget.issues[field]),
        onChanged: change,
      ),
    );
    void secondaryChange(int index, int id) {
      final next = List<int>.from(secondary);
      while (next.length <= index) {
        next.add(0);
      }
      next[index] = id;
      set('secondary_category_ids', next.where((n) => n > 0).toList());
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if ((d['category_id'] ?? 0) > 0 &&
            (d[widget.kind == 'vendor'
                        ? 'primary_category_id'
                        : 'product_category_id'] ??
                    0) ==
                0)
          Text(
            zh
                ? '历史分类已保留，可重新选择分类。'
                : 'Original classification retained. You may select a new category.',
          ),
        if (widget.kind == 'vendor') ...[
          picker(
            'primary_category_id',
            d['primary_category_id'] ?? 0,
            zh ? '主营分类' : 'Primary business category',
            (v) => set('primary_category_id', v),
            secondary,
          ),
          for (var i = 0; i < 2; i++)
            picker(
              'secondary_category_ids',
              secondary.length > i ? secondary[i] : 0,
              zh
                  ? '兼营分类${i + 1}（选填）'
                  : 'Secondary category ${i + 1} (optional)',
              (v) => secondaryChange(i, v),
              [
                d['primary_category_id'] ?? 0,
                ...secondary
                    .asMap()
                    .entries
                    .where((e) => e.key != i)
                    .map((e) => e.value),
              ],
            ),
          DropdownButtonFormField<String>(
            key: ValueKey('nature:${d['business_nature']}'),
            initialValue:
                nativeBusinessNatures.containsKey(d['business_nature'])
                ? d['business_nature']
                : '',
            isExpanded: true,
            decoration: InputDecoration(
              labelText: zh ? '经营性质' : 'Business nature',
            ),
            items: [
              DropdownMenuItem(value: '', child: Text(m.t('optional'))),
              for (final e in nativeBusinessNatures.entries)
                DropdownMenuItem(
                  value: e.key,
                  child: Text(e.value[zh ? 0 : 1]),
                ),
            ],
            onChanged: widget.enabled
                ? (v) => set('business_nature', v ?? '')
                : null,
          ),
        ] else ...[
          picker(
            'product_category_id',
            d['product_category_id'] ?? 0,
            zh ? '产品分类' : 'Product category',
            (v) => set('product_category_id', v),
            [],
          ),
          OutlinedButton(
            onPressed:
                widget.enabled &&
                    !recommending &&
                    (d['name'] ?? '').toString().trim().isNotEmpty
                ? recommend
                : null,
            child: Text(zh ? '推荐分类' : 'Recommend categories'),
          ),
          for (final entry in {
            'brand': zh ? '产品品牌' : 'Product brand',
            'model': zh ? '产品型号' : 'Product model',
            'applicable_brands': zh ? '适用品牌' : 'Compatible brands',
            'applicable_models': zh ? '适用机型' : 'Compatible models',
          }.entries)
            Padding(
              padding: const EdgeInsets.only(top: 12, bottom: 4),
              child: TextField(
                controller: controllers[entry.key],
                enabled: widget.enabled,
                maxLength: entry.key.startsWith('applicable_') ? 6030 : 200,
                decoration: InputDecoration(
                  labelText: entry.value,
                  counterText: '',
                  helperText: entry.key.startsWith('applicable_')
                      ? (zh
                            ? '多项用逗号分隔，最多30项'
                            : 'Separate up to 30 entries with commas')
                      : null,
                  errorText: widget.issues[entry.key] == null
                      ? null
                      : fieldIssueText(m, asMap(widget.issues[entry.key])),
                ),
                onChanged: (v) => set(
                  entry.key,
                  entry.key.startsWith('applicable_') ? tokens(v) : v,
                ),
              ),
            ),
        ],
        if ((d['industrial_belt_ids'] as List? ?? []).isNotEmpty)
          NativeTaxonomySummary(data: d, kind: widget.kind, beltsOnly: true),
        space(),
      ],
    );
  }
}

class NativeTaxonomySummary extends StatelessWidget {
  final Json data;
  final String kind;
  final bool beltsOnly;
  const NativeTaxonomySummary({
    super.key,
    required this.data,
    required this.kind,
    this.beltsOnly = false,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    return AsyncPane(
      identity:
          '${m.baseUrl}:taxonomy-summary:${jsonEncode(data)}:${m.language}',
      load: () => m.publicTaxonomy(kind: 'all'),
      builder: (result) {
        final cats = rows(result['categories']),
            belts = rows(result['industrial_belts']);
        final details = <String, String>{};
        if (!beltsOnly) {
          final id =
              data[kind == 'vendor'
                  ? 'primary_category_id'
                  : 'product_category_id'] ??
              0;
          if (id > 0) {
            details[zh
                ? (kind == 'vendor' ? '主营分类' : '产品分类')
                : (kind == 'vendor'
                      ? 'Primary category'
                      : 'Product category')] = nativeCategoryPath(
              cats,
              id,
              m,
            );
          }
          final secondary = data['secondary_category_ids'] as List? ?? [];
          if (secondary.isNotEmpty) {
            details[zh ? '兼营分类' : 'Secondary categories'] = secondary
                .map((id) => nativeCategoryPath(cats, id, m))
                .join('、');
          }
          final nature = nativeBusinessNatures[data['business_nature']];
          if (nature != null) {
            details[zh ? '经营性质' : 'Business nature'] = nature[zh ? 0 : 1];
          }
          for (final entry in {
            'brand': zh ? '产品品牌' : 'Product brand',
            'model': zh ? '产品型号' : 'Product model',
            'applicable_brands': zh ? '适用品牌' : 'Compatible brands',
            'applicable_models': zh ? '适用机型' : 'Compatible models',
          }.entries) {
            final value = data[entry.key];
            final text = value is List
                ? value.join('、')
                : (value ?? '').toString();
            if (text.isNotEmpty) details[entry.value] = text;
          }
        }
        final beltIDs = data['industrial_belt_ids'] as List? ?? [];
        if (beltIDs.isNotEmpty) {
          details[zh ? '产业带（后台维护）' : 'Industrial belts'] = beltIDs
              .map((id) {
                final b = belts.where((v) => v['id'] == id).firstOrNull;
                return b == null ? '#$id' : m.content(b, 'name');
              })
              .join('、');
        }
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (final entry in details.entries)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text('${entry.key}：${entry.value}'),
              ),
          ],
        );
      },
    );
  }
}
