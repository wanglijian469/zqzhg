part of 'main.dart';

Future<Json?> editVendorProductCategory(
  BuildContext context,
  Json catalog, {
  Json? value,
}) async {
  return showDialog<Json>(
    context: context,
    builder: (_) => _VendorCategoryDialog(catalog: catalog, value: value),
  );
}

class _VendorCategoryDialog extends StatefulWidget {
  final Json catalog;
  final Json? value;
  const _VendorCategoryDialog({required this.catalog, this.value});
  @override
  State<_VendorCategoryDialog> createState() => _VendorCategoryDialogState();
}

class _VendorCategoryDialogState extends State<_VendorCategoryDialog> {
  late final name = TextEditingController(text: widget.value?['name'] ?? '');
  late final english = TextEditingController(
    text: widget.value?['name_en'] ?? '',
  );
  late final order = TextEditingController(
    text: '${widget.value?['sort_order'] ?? 0}',
  );
  late int root;
  late bool enabled;
  bool busy = false;
  String error = '';
  @override
  void initState() {
    super.initState();
    final preferred = (widget.catalog['preferred_industry_ids'] as List? ?? []);
    final roots = rows(widget.catalog['industry_categories']);
    root =
        widget.value?['industry_category_id'] ??
        preferred.where((id) => roots.any((r) => r['id'] == id)).firstOrNull ??
        0;
    enabled = widget.value?['enabled'] ?? true;
  }

  @override
  void dispose() {
    name.dispose();
    english.dispose();
    order.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    final preferred = widget.catalog['preferred_industry_ids'] as List? ?? [];
    final roots = rows(widget.catalog['industry_categories']);
    roots.sort((a, b) {
      final x = preferred.indexOf(a['id']), y = preferred.indexOf(b['id']);
      return (x < 0 ? 999 : x).compareTo(y < 0 ? 999 : y);
    });
    return PopScope(
      canPop: !busy,
      child: AlertDialog(
        title: Text(
          zh
              ? (widget.value == null ? '新增产品分类' : '编辑产品分类')
              : (widget.value == null
                    ? 'Add product category'
                    : 'Edit product category'),
        ),
        content: SizedBox(
          width: 480,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                TextField(
                  controller: name,
                  enabled: !busy,
                  decoration: InputDecoration(
                    labelText: zh ? '分类名称' : 'Category name',
                  ),
                ),
                space(12),
                TextField(
                  controller: english,
                  enabled: !busy,
                  decoration: InputDecoration(
                    labelText: zh ? '英文名称（选填）' : 'English name (optional)',
                  ),
                ),
                space(12),
                DropdownButtonFormField<int>(
                  initialValue: roots.any((r) => r['id'] == root) ? root : 0,
                  isExpanded: true,
                  decoration: InputDecoration(
                    labelText: zh ? '所属行业大类' : 'Industry category',
                  ),
                  items: [
                    DropdownMenuItem(
                      value: 0,
                      child: Text(zh ? '请选择' : 'Choose'),
                    ),
                    for (final r in roots)
                      DropdownMenuItem<int>(
                        value: r['id'],
                        child: Text(
                          m.content(r, 'name'),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                  ],
                  onChanged: busy ? null : (v) => setState(() => root = v ?? 0),
                ),
                space(12),
                Text(
                  zh
                      ? '仅用于本企业，保存后立即可用。产品内容仍需审核。名称建议使用产品类型或系列。'
                      : 'Only your company uses this category. Available immediately; products still require review. Use a product type or series name.',
                ),
                if (widget.value != null) ...[
                  space(12),
                  TextField(
                    controller: order,
                    enabled: !busy,
                    keyboardType: const TextInputType.numberWithOptions(
                      signed: true,
                    ),
                    decoration: InputDecoration(
                      labelText: zh ? '排序' : 'Sort order',
                    ),
                  ),
                  SwitchListTile(
                    contentPadding: EdgeInsets.zero,
                    title: Text(
                      zh ? '允许新产品选择' : 'Available for new selections',
                    ),
                    value: enabled,
                    onChanged: busy ? null : (v) => setState(() => enabled = v),
                  ),
                ],
                if (error.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 12),
                    child: Text(
                      error,
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.error,
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: busy ? null : () => Navigator.pop(context),
            child: Text(m.t('cancel')),
          ),
          FilledButton(
            onPressed: busy
                ? null
                : () async {
                    if (name.text.trim().isEmpty ||
                        root == 0 ||
                        utf8.encode(name.text.trim()).length > 300 ||
                        utf8.encode(english.text.trim()).length > 300) {
                      setState(
                        () => error = zh
                            ? '请填写名称（最多300字节）并选择行业大类。'
                            : 'Enter a name (up to 300 bytes) and select an industry.',
                      );
                      return;
                    }
                    setState(() {
                      busy = true;
                      error = '';
                    });
                    try {
                      final saved = asMap(
                        await m.api(
                          '/merchant/product-categories${widget.value == null ? '' : '/${widget.value!['id']}'}',
                          method: widget.value == null ? 'POST' : 'PUT',
                          data: {
                            'name': name.text.trim(),
                            'name_en': english.text.trim(),
                            'industry_category_id': root,
                            'sort_order': int.tryParse(order.text) ?? 0,
                            'enabled': enabled,
                          },
                        ),
                      );
                      if (!mounted || !context.mounted) return;
                      m.rememberVendorProductCategories([saved]);
                      Navigator.pop(context, saved);
                    } catch (e) {
                      if (mounted) {
                        setState(
                          () => error = e is ApiFailure
                              ? m.t(e.code)
                              : m.t('network_error'),
                        );
                      }
                    } finally {
                      if (mounted) setState(() => busy = false);
                    }
                  },
            child: Text(zh ? '保存分类' : 'Save category'),
          ),
        ],
      ),
    );
  }
}

class NativeVendorProductCategoryPicker extends StatefulWidget {
  final int value;
  final ValueChanged<int> onChanged;
  final bool enabled;
  final Json? issue;
  const NativeVendorProductCategoryPicker({
    super.key,
    required this.value,
    required this.onChanged,
    this.enabled = true,
    this.issue,
  });
  @override
  State<NativeVendorProductCategoryPicker> createState() =>
      _NativeVendorProductCategoryPickerState();
}

class _NativeVendorProductCategoryPickerState
    extends State<NativeVendorProductCategoryPicker> {
  int generation = 0;
  String query = '';
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    return AsyncPane(
      identity:
          '${m.scope}:${m.token.hashCode}:vendor-product-categories:$generation',
      load: () => m.api('/merchant/product-categories'),
      builder: (catalog) {
        final cats = rows(catalog['items']);
        m.rememberVendorProductCategories(cats);
        final choices = cats
            .where(
              (c) =>
                  ((c['enabled'] == true && c['needs_industry'] != true) ||
                      c['id'] == widget.value) &&
                  (c['id'] == widget.value ||
                      [c['name'], c['name_en']].any(
                        (v) => (v ?? '').toString().toLowerCase().contains(
                          query.toLowerCase(),
                        ),
                      )),
            )
            .toList();
        String categoryLabel(Json c) {
          final name = m.content(c, 'name');
          if (cats.where((other) => m.content(other, 'name') == name).length >
              1) {
            final industry = m.content(c, 'industry_name');
            return '$name · ${industry.isEmpty ? (zh ? '待完善归属' : 'Industry needs attention') : industry}';
          }
          return name;
        }

        final selected = choices.any((c) => c['id'] == widget.value)
            ? widget.value
            : 0;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            NativeFieldLabel(
              label: zh ? '本企业产品分类' : 'Company product category',
              child: DropdownButtonFormField<int>(
                key: ValueKey('$selected:$generation'),
                initialValue: selected,
                isExpanded: true,
                decoration: InputDecoration(
                  errorText: widget.issue == null
                      ? null
                      : fieldIssueText(m, widget.issue!),
                ),
                items: [
                  DropdownMenuItem(
                    value: 0,
                    child: Text(zh ? '请选择产品分类' : 'Choose a product category'),
                  ),
                  for (final c in choices)
                    DropdownMenuItem<int>(
                      value: c['id'],
                      child: Text(
                        '${categoryLabel(c)}${c['enabled'] == true
                            ? ''
                            : zh
                            ? '（已停用）'
                            : ' (inactive)'}',
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                ],
                onChanged: widget.enabled
                    ? (v) => widget.onChanged(v ?? 0)
                    : null,
              ),
            ),
            space(8),
            TextField(
              enabled: widget.enabled,
              decoration: InputDecoration(
                labelText: zh
                    ? '搜索本企业产品分类'
                    : 'Search company product categories',
              ),
              onChanged: (v) => setState(() => query = v),
            ),
            space(8),
            OutlinedButton.icon(
              icon: const Icon(Icons.add),
              onPressed: widget.enabled
                  ? () async {
                      final saved = await editVendorProductCategory(
                        context,
                        asMap(catalog),
                      );
                      if (!mounted || saved == null) return;
                      widget.onChanged(saved['id']);
                      setState(() => generation++);
                    }
                  : null,
              label: Text(zh ? '新增产品分类' : 'Add product category'),
            ),
            if (cats.isEmpty)
              Text(
                zh
                    ? '还没有分类，可以先新增一个。'
                    : 'No company categories yet. Add one to get started.',
              ),
          ],
        );
      },
    );
  }
}

class VendorProductCategoriesScreen extends StatefulWidget {
  const VendorProductCategoriesScreen({super.key});
  @override
  State<VendorProductCategoriesScreen> createState() =>
      _VendorProductCategoriesScreenState();
}

class _VendorProductCategoriesScreenState
    extends State<VendorProductCategoriesScreen> {
  int generation = 0;
  String query = '';
  bool busy = false;
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    return MerchantScaffold(
      title: m.t('vendorProductCategories'),
      body: AsyncPane(
        identity:
            '${m.scope}:${m.token.hashCode}:manage-product-categories:$generation',
        load: () => m.api('/merchant/product-categories'),
        builder: (catalog) {
          final cats = rows(catalog['items']);
          m.rememberVendorProductCategories(cats);
          Future<void> edit([Json? value]) async {
            final saved = await editVendorProductCategory(
              context,
              asMap(catalog),
              value: value,
            );
            if (mounted && saved != null) setState(() => generation++);
          }

          return ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Text(
                zh
                    ? '仅管理本企业分类。停用后保留已有产品引用；有产品版本引用时不能删除。'
                    : 'Manage only your company categories. Inactive categories keep existing assignments. Referenced categories cannot be deleted.',
              ),
              space(12),
              FilledButton.icon(
                onPressed: busy ? null : () => edit(),
                icon: const Icon(Icons.add),
                label: Text(zh ? '新增产品分类' : 'Add product category'),
              ),
              space(12),
              TextField(
                decoration: InputDecoration(
                  labelText: zh ? '检索产品分类' : 'Find product categories',
                ),
                onChanged: (v) => setState(() => query = v),
              ),
              space(12),
              for (final cat in cats.where(
                (c) => [c['name'], c['name_en'], c['industry_name']].any(
                  (v) => (v ?? '').toString().toLowerCase().contains(
                    query.toLowerCase(),
                  ),
                ),
              ))
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          m.content(cat, 'name'),
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        space(4),
                        Text(
                          cat['needs_industry'] == true
                              ? (zh ? '待完善归属' : 'Industry needs attention')
                              : m.content(cat, 'industry_name'),
                        ),
                        Text(
                          '${zh ? '排序' : 'Order'} ${cat['sort_order']} · ${cat['enabled'] == true ? (zh ? '启用' : 'Active') : (zh ? '已停用' : 'Inactive')}',
                        ),
                        Wrap(
                          spacing: 12,
                          children: [
                            TextButton(
                              onPressed: busy ? null : () => edit(cat),
                              child: Text(m.t('edit')),
                            ),
                            TextButton(
                              onPressed: busy
                                  ? null
                                  : () async {
                                      final confirmed = await showDialog<bool>(
                                        context: context,
                                        builder: (ctx) => AlertDialog(
                                          title: Text(
                                            zh
                                                ? '删除此分类？'
                                                : 'Delete this category?',
                                          ),
                                          content: Text(
                                            zh
                                                ? '已有产品版本引用时无法删除。'
                                                : 'Referenced categories cannot be deleted.',
                                          ),
                                          actions: [
                                            TextButton(
                                              onPressed: () =>
                                                  Navigator.pop(ctx, false),
                                              child: Text(m.t('cancel')),
                                            ),
                                            TextButton(
                                              onPressed: () =>
                                                  Navigator.pop(ctx, true),
                                              child: Text(zh ? '删除' : 'Delete'),
                                            ),
                                          ],
                                        ),
                                      );
                                      if (!mounted || confirmed != true) return;
                                      setState(() => busy = true);
                                      try {
                                        await m.api(
                                          '/merchant/product-categories/${cat['id']}',
                                          method: 'DELETE',
                                        );
                                        if (mounted) {
                                          setState(() => generation++);
                                        }
                                      } catch (e) {
                                        if (context.mounted) {
                                          showError(context, e);
                                        }
                                      } finally {
                                        if (mounted) {
                                          setState(() => busy = false);
                                        }
                                      }
                                    },
                              child: Text(zh ? '删除' : 'Delete'),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ),
              if (cats.isEmpty)
                Text(zh ? '暂无产品分类' : 'No product categories yet'),
            ],
          );
        },
      ),
    );
  }
}
