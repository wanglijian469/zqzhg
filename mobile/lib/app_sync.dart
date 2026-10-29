part of 'main.dart';

String syncText(BuildContext context, String zh, String en) =>
    ModelScope.of(context).language == 'zh' ? zh : en;

Future<bool> nativeSessionReady(
  BuildContext context,
  int owner,
  String baseUrl,
) async {
  final m = ModelScope.of(context);
  if (m.baseUrl != baseUrl ||
      (m.token.isNotEmpty && owner != 0 && m.account['id'] != owner)) {
    return false;
  }
  if (m.token.isNotEmpty) return true;
  final ok = await Navigator.push<bool>(
    context,
    MaterialPageRoute(builder: (_) => const LoginScreen()),
  );
  if (!context.mounted ||
      ok != true ||
      m.baseUrl != baseUrl ||
      (owner != 0 && m.account['id'] != owner)) {
    return false;
  }
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text(
        syncText(
          context,
          '登录成功，资料已保留，请重试原操作。',
          'Signed in. Your edits are retained; retry your operation.',
        ),
      ),
    ),
  );
  return false;
}

Json credentialEditorData(Json data) {
  final p = asMap(data['company_profile'] ?? {});
  if (p['credentials'] == null) return data;
  return {
    ...data,
    'company_profile': {
      ...p,
      'credentials': rows(p['credentials'])
          .asMap()
          .entries
          .map(
            (e) => <String, dynamic>{
              ...e.value,
              '_key':
                  e.value['_key'] ?? 'existing-${e.key}-${e.value['media_id']}',
            },
          )
          .toList(),
    },
  };
}

List<Json> companyGallery(Json data) {
  final annotations = rows(
    asMap(data['company_profile'] ?? {})['gallery_annotations'],
  );
  return imageIDs(data)
      .map(
        (id) =>
            annotations.where((r) => r['media_id'] == id).firstOrNull ??
            <String, dynamic>{'media_id': id},
      )
      .toList();
}

class CompanyPortalBody extends StatelessWidget {
  final Json data;
  final Json? result;
  final bool compact;
  final List<Json> localPhotos;
  final ValueChanged<int>? onTab;
  const CompanyPortalBody({
    super.key,
    required this.data,
    this.result,
    this.compact = false,
    this.onTab,
    this.localPhotos = const [],
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), p = asMap(data['company_profile'] ?? {});
    final gallery = companyGallery(data);
    final credentials = rows(p['credentials']);
    final category = asMap(result?['business_category'] ?? {});
    final description = compact
        ? m.content(p, 'summary')
        : m.content(data, 'description');
    final overview = description.isEmpty
        ? m.content(data, 'description')
        : description;
    Widget heading(String text) => Padding(
      padding: const EdgeInsets.only(top: 24, bottom: 12),
      child: Text(text, style: Theme.of(context).textTheme.titleLarge),
    );
    Widget images(List<Json> list) => Column(
      children: list.map((r) {
        final local = localPhotos
            .where(
              (f) => f['slot'] == 'credential:${r['_key']}' && f['id'] == null,
            )
            .firstOrNull;
        return Card(
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (local != null)
                  Image.file(
                    File(local['path']),
                    height: 200,
                    fit: BoxFit.contain,
                  )
                else if ((r['media_id'] ?? 0) > 0)
                  InkWell(
                    onTap: () => push(
                      context,
                      PhotoViewer(
                        ids: list
                            .map((r) => r['media_id'] as int)
                            .where((id) => id > 0)
                            .toList(),
                        initial: list
                            .where((r) => (r['media_id'] ?? 0) > 0)
                            .toList()
                            .indexOf(r),
                      ),
                    ),
                    child: Photo(
                      id: r['media_id'],
                      thumb: false,
                      fit: BoxFit.contain,
                    ),
                  ),
                if (m.content(r, 'title').isNotEmpty)
                  Text(
                    m.content(r, 'title'),
                    style: const TextStyle(fontWeight: FontWeight.bold),
                  ),
                if (m.content(r, 'description').isNotEmpty)
                  Text(m.content(r, 'description')),
              ],
            ),
          ),
        );
      }).toList(),
    );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        heading(m.t('companyIntro')),
        if (overview.isNotEmpty)
          Text(
            compact && overview.runes.length > 240
                ? '${String.fromCharCodes(overview.runes.take(240))}…'
                : overview,
          ),
        for (final f in <String, String>{
          'company': m.content(data, 'name'),
          'mainProducts': (data['main_products'] ?? '').toString(),
          'category': m.content(category, 'name'),
          'regionName': (data['region_name'] ?? '').toString(),
          'established': (p['established'] ?? '').toString(),
          'staff': (p['staff'] ?? '').toString(),
          'companyAddress': (data['address'] ?? '').toString(),
        }.entries)
          if (f.value.trim().isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 8),
              child: Text('${m.t(f.key)}：${f.value}'),
            ),
        if (compact && rows(result?['representative_products']).isNotEmpty) ...[
          heading(m.t('representativeProducts')),
          ExhibitGrid(items: rows(result?['representative_products'])),
        ],
        if ([
          'factory_area',
          'equipment',
          'capacity',
        ].any((k) => m.content(p, k).trim().isNotEmpty)) ...[
          heading(m.t('factoryFacts')),
          for (final k in ['factory_area', 'equipment', 'capacity'])
            if (m.content(p, k).trim().isNotEmpty)
              Text('${m.t(k)}：${m.content(p, k)}'),
        ],
        for (final kind in ['qualification', 'award'])
          if (credentials.any((r) => r['kind'] == kind)) ...[
            heading(
              syncText(
                context,
                kind == 'qualification' ? '企业资质' : '获奖荣誉',
                kind == 'qualification' ? 'Qualifications' : 'Honors & awards',
              ),
            ),
            images(
              credentials
                  .where((r) => r['kind'] == kind)
                  .take(compact ? 4 : 20)
                  .toList(),
            ),
          ],
        if (gallery.isNotEmpty ||
            localPhotos.any((f) => f['slot'] == null && f['id'] == null)) ...[
          heading(syncText(context, '工厂与产品实拍', 'Factory gallery')),
          images(gallery.take(compact ? 6 : 12).toList()),
          for (final f in localPhotos.where(
            (f) => f['slot'] == null && f['id'] == null,
          ))
            Image.file(
              File(f['path']),
              height: 200,
              fit: BoxFit.contain,
              errorBuilder: (_, _, _) => Text(m.t('photoRetry')),
            ),
        ],
        if (compact && onTab != null) ...[
          for (final entry in [
            ('video', 2, 'shortVideos'),
            ('catalog', 3, 'electronicCatalogs'),
          ])
            if ((asMap(result?['resource_counts'] ?? {})[entry.$1] ?? 0) >
                0) ...[
              heading(m.t(entry.$3)),
              PortalResourceSummary(
                vendorID: result!['item']['id'],
                kind: entry.$1,
              ),
              TextButton(
                onPressed: () => onTab!(entry.$2),
                child: Text(m.t(entry.$3)),
              ),
            ],
          heading(m.t('showroomContact')),
          Contact(data: data),
          OutlinedButton(
            onPressed: () => onTab!(4),
            child: Text(m.t('showroomContact')),
          ),
        ],
      ],
    );
  }
}

class PortalResourceSummary extends StatelessWidget {
  final int vendorID;
  final String kind;
  const PortalResourceSummary({
    super.key,
    required this.vendorID,
    required this.kind,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return AsyncPane(
      identity: '${m.baseUrl}:portal-resources:$vendorID:$kind',
      load: () => m.api(
        '/public/resources?vendor_id=$vendorID&kind=$kind&page_size=3&page=1',
      ),
      builder: (r) => Column(
        children: rows(r['items'])
            .where((item) => item['vendor_id'] == vendorID)
            .take(3)
            .map((item) => NativePublicResourceCard(item: item))
            .toList(),
      ),
    );
  }
}

class RepresentativeSettingsScreen extends StatefulWidget {
  const RepresentativeSettingsScreen({super.key});
  @override
  State<RepresentativeSettingsScreen> createState() =>
      _RepresentativeSettingsScreenState();
}

class _RepresentativeSettingsScreenState
    extends State<RepresentativeSettingsScreen> {
  String mode = 'auto', query = '';
  int version = 0, page = 1, reload = 0, vendorID = 0;
  bool ready = false, busy = false;
  String error = '';
  List<int> ids = [];
  final Map<int, Json> products = {};
  List<Json> preview = [];
  final search = TextEditingController();
  int? owner;
  String? sourceEndpoint;
  bool retired = false;
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    owner ??= m.account['id'];
    sourceEndpoint ??= m.baseUrl;
    if (!retired &&
        (m.baseUrl != sourceEndpoint ||
            (m.token.isNotEmpty &&
                owner != null &&
                m.account['id'] != owner))) {
      retired = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) Navigator.pop(context);
      });
    }
  }

  @override
  void dispose() {
    search.dispose();
    super.dispose();
  }

  Future<void> load({bool retain = false}) async {
    if (retired) return;
    final m = ModelScope.of(context);
    setState(() => busy = true);
    try {
      final r = asMap(await m.api('/merchant/showroom-display'));
      if (!mounted) return;
      setState(() {
        version = r['version'];
        vendorID = r['vendor_id'];
        if (!retain) {
          mode = r['mode'];
          ids = List<int>.from(r['product_ids'] ?? []);
        }
        for (final row in rows(r['selected'])) {
          products[row['product_id']] = row['item'] is Map
              ? asMap(row['item'])
              : {
                  'data': {'name': '#${row['product_id']}'},
                  'public_visible': false,
                };
        }
        preview = rows(r['preview_products']);
        ready = true;
        error = retain
            ? syncText(
                context,
                '已刷新版本，选择已保留，请核对后保存。',
                'Version refreshed. Check retained choices before saving.',
              )
            : '';
      });
    } catch (e) {
      if (mounted) {
        setState(
          () => error = e is ApiFailure ? m.t(e.code) : m.t('network_error'),
        );
      }
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> save() async {
    if (retired) return;
    final m = ModelScope.of(context);
    setState(() => busy = true);
    try {
      await m.api(
        '/merchant/showroom-display',
        method: 'PUT',
        data: {
          'mode': mode,
          'product_ids': mode == 'auto' ? [] : ids,
          'version': version,
        },
      );
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(m.t('saved'))));
        await load();
      }
    } catch (e) {
      if (mounted) {
        setState(
          () => error = e is ApiFailure ? m.t(e.code) : m.t('network_error'),
        );
      }
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (!ready && !busy && error.isEmpty) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) load();
      });
    }
    return Scaffold(
      appBar: appBar(
        context,
        syncText(context, '设置精选产品', 'Select featured products'),
      ),
      body: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          Text(
            syncText(
              context,
              '最多3款本厂公开产品；与首页精选独立。手动清空后隐藏代表展品。',
              'Up to 3 published company products, independent of homepage selections. Empty manual selection hides this section.',
            ),
          ),
          if (error.isNotEmpty) message(context, error),
          if (m.token.isEmpty)
            OutlinedButton(
              onPressed: busy
                  ? null
                  : () async {
                      await Navigator.push<bool>(
                        context,
                        MaterialPageRoute(builder: (_) => const LoginScreen()),
                      );
                      if (!context.mounted) return;
                      if (m.account['id'] != owner) {
                        Navigator.pop(context);
                        return;
                      }
                      setState(
                        () => error = syncText(
                          context,
                          '登录成功，请重试原操作。',
                          'Signed in. Retry your operation.',
                        ),
                      );
                    },
              child: Text(m.t('login')),
            ),
          if (!ready)
            OutlinedButton(
              onPressed: busy ? null : load,
              child: Text(m.t('retry')),
            ),
          if (ready) ...[
            DropdownButtonFormField<String>(
              initialValue: mode,
              items: [
                DropdownMenuItem(
                  value: 'auto',
                  child: Text(
                    syncText(context, '自动推荐', 'Automatic recommendations'),
                  ),
                ),
                DropdownMenuItem(
                  value: 'manual',
                  child: Text(syncText(context, '手动选择', 'Manual selection')),
                ),
              ],
              onChanged: busy
                  ? null
                  : (v) => setState(() {
                      mode = v!;
                      if (mode == 'auto') ids = [];
                    }),
            ),
            if (mode == 'auto') ExhibitGrid(items: preview),
            if (mode == 'manual') ...[
              for (var i = 0; i < ids.length; i++)
                Card(
                  child: Column(
                    children: [
                      Text(
                        products[ids[i]] == null
                            ? '#${ids[i]}'
                            : m.content(
                                asMap(products[ids[i]]!['data']),
                                'name',
                              ),
                      ),
                      if (products[ids[i]]?['public_visible'] == false)
                        Text(
                          syncText(
                            context,
                            '已失效，请移除后保存；下架后停止公开。',
                            'Unavailable. Remove before saving.',
                          ),
                        ),
                      Wrap(
                        children: [
                          IconButton(
                            tooltip: syncText(context, '上移', 'Move up'),
                            onPressed: busy || i == 0
                                ? null
                                : () => setState(() {
                                    final id = ids.removeAt(i);
                                    ids.insert(i - 1, id);
                                  }),
                            icon: const Icon(Icons.arrow_upward),
                          ),
                          IconButton(
                            tooltip: syncText(context, '下移', 'Move down'),
                            onPressed: busy || i == ids.length - 1
                                ? null
                                : () => setState(() {
                                    final id = ids.removeAt(i);
                                    ids.insert(i + 1, id);
                                  }),
                            icon: const Icon(Icons.arrow_downward),
                          ),
                          TextButton(
                            onPressed: busy
                                ? null
                                : () => setState(() => ids.removeAt(i)),
                            child: Text(m.t('remove')),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              NativeFieldLabel(
                label: m.t('productSearch'),
                child: TextField(
                  controller: search,
                  decoration: InputDecoration(),
                  onSubmitted: (s) => setState(() {
                    query = s;
                    page = 1;
                    reload++;
                  }),
                ),
              ),
              TextButton(
                onPressed: busy
                    ? null
                    : () => setState(() {
                        query = search.text;
                        page = 1;
                        reload++;
                      }),
                child: Text(m.t('search')),
              ),
              AsyncPane(
                identity: '${m.scope}:representatives:$query:$page:$reload',
                load: () => m.api(
                  '/public/products?vendor_id=$vendorID&page_size=20&page=$page&q=${Uri.encodeComponent(query)}',
                ),
                builder: (r) => Column(
                  children: [
                    for (final item in rows(r['items']))
                      CheckboxListTile(
                        title: Text(
                          m.content(
                            asMap(item['public_data'] ?? item['data']),
                            'name',
                          ),
                        ),
                        subtitle: Text(
                          item['public_visible'] == true
                              ? m.t('published')
                              : m.t('unpublished'),
                        ),
                        value: ids.contains(item['id']),
                        onChanged:
                            busy ||
                                item['public_visible'] != true ||
                                (!ids.contains(item['id']) && ids.length >= 3)
                            ? null
                            : (v) => setState(() {
                                products[item['id']] = {
                                  ...item,
                                  'data': item['public_data'] ?? item['data'],
                                };
                                if (v == true) {
                                  ids.add(item['id']);
                                } else {
                                  ids.remove(item['id']);
                                }
                              }),
                      ),
                    Wrap(
                      children: [
                        TextButton(
                          onPressed: page > 1
                              ? () => setState(() => page--)
                              : null,
                          child: Text(m.t('previous')),
                        ),
                        Text('$page'),
                        TextButton(
                          onPressed: page * 20 < (r['total'] ?? 0)
                              ? () => setState(() => page++)
                              : null,
                          child: Text(m.t('next')),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            ],
            OutlinedButton(
              onPressed: busy ? null : () => load(retain: true),
              child: Text(
                syncText(context, '刷新配置版本', 'Refresh configuration version'),
              ),
            ),
            FilledButton(
              onPressed: busy || m.token.isEmpty ? null : save,
              child: Text(
                syncText(context, '保存设置精选产品', 'Save featured products'),
              ),
            ),
          ],
          if (busy) const Center(child: CircularProgressIndicator()),
        ],
      ),
    );
  }
}

class PortalMediaEditor extends StatelessWidget {
  final bool initiallyExpanded;
  final Json data;
  final bool enabled;
  final ValueChanged<Json> onChanged;
  final List<Json> photos;
  final Future<void> Function(String, XFile) onPick;
  final Future<void> Function() onRetry;
  final ValueChanged<String> onDiscard;
  final Map<String, Json> issues;
  const PortalMediaEditor({
    super.key,
    this.initiallyExpanded = false,
    required this.data,
    required this.enabled,
    required this.onChanged,
    required this.photos,
    required this.onPick,
    required this.onRetry,
    required this.onDiscard,
    this.issues = const {},
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), p = asMap(data['company_profile'] ?? {});
    final credentials = rows(p['credentials']), gallery = companyGallery(data);
    void update(String key, dynamic value) => onChanged({
      ...data,
      'company_profile': {...p, key: value},
    });
    void change(int index, Json patch) {
      final next = credentials.map((r) => <String, dynamic>{...r}).toList();
      next[index].addAll(patch);
      update('credentials', next);
    }

    Widget fields(
      Json r,
      String identity,
      ValueChanged<Json> change, {
      bool requiredTitle = false,
      String? issuePrefix,
    }) => Column(
      children: [
        for (final k in ['title', 'description', 'title_en', 'description_en'])
          NativeFieldLabel(
            label:
                '${k.startsWith('title') ? syncText(context, '标题', 'Title') : syncText(context, '说明', 'Description')}${k.endsWith('_en') ? ' (English)' : ''}${requiredTitle && k == 'title' ? ' *' : ''}',
            child: TextFormField(
              key: ValueKey('$identity:$k'),
              initialValue: (r[k] ?? '').toString(),
              enabled: enabled,
              maxLines: k.startsWith('description') ? 3 : 1,
              decoration: InputDecoration(
                errorText: issues['$issuePrefix.$k'] == null
                    ? null
                    : fieldIssueText(m, issues['$issuePrefix.$k']!),

                helperText: k.startsWith('title')
                    ? syncText(context, '最多100字', 'Up to 100 characters')
                    : syncText(context, '最多500字', 'Up to 500 characters'),
              ),
              onChanged: (v) => change({k: v}),
            ),
          ),
      ],
    );
    Widget ordering(int index, int length, void Function(int delta) move) =>
        Wrap(
          children: [
            IconButton(
              tooltip: syncText(context, '上移', 'Move up'),
              onPressed: enabled && index > 0 ? () => move(-1) : null,
              icon: const Icon(Icons.arrow_upward),
            ),
            IconButton(
              tooltip: syncText(context, '下移', 'Move down'),
              onPressed: enabled && index < length - 1 ? () => move(1) : null,
              icon: const Icon(Icons.arrow_downward),
            ),
          ],
        );
    return ExpansionTile(
      key: ValueKey(
        'portal-errors-${issues.keys.where((k) => k.contains('credentials') || k.contains('gallery_annotations')).join(',')}',
      ),
      initiallyExpanded:
          initiallyExpanded ||
          issues.keys.any(
            (k) =>
                k.contains('credentials') || k.contains('gallery_annotations'),
          ),
      title: Text(
        syncText(context, '资质荣誉与图集标注', 'Credentials & gallery labels'),
      ),
      childrenPadding: const EdgeInsets.all(12),
      children: [
        for (var i = 0; i < credentials.length; i++)
          Builder(
            builder: (context) {
              final row = credentials[i],
                  slot = 'credential:${credentials[i]['_key']}';
              final local = photos
                  .where((f) => f['slot'] == slot && f['id'] == null)
                  .firstOrNull;
              return Card(
                child: Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    children: [
                      DropdownButtonFormField<String>(
                        key: ValueKey('$slot:kind'),
                        initialValue: row['kind'] ?? 'qualification',
                        items: [
                          DropdownMenuItem(
                            value: 'qualification',
                            child: Text(
                              syncText(context, '企业资质', 'Qualification'),
                            ),
                          ),
                          DropdownMenuItem(
                            value: 'award',
                            child: Text(syncText(context, '获奖荣誉', 'Award')),
                          ),
                        ],
                        onChanged: enabled
                            ? (v) => change(i, {'kind': v})
                            : null,
                      ),
                      if (local != null)
                        Image.file(
                          File(local['path']),
                          height: 160,
                          fit: BoxFit.contain,
                          errorBuilder: (_, _, _) => Text(m.t('photoRetry')),
                        )
                      else if ((row['media_id'] ?? 0) > 0)
                        Photo(id: row['media_id'], height: 160),
                      OutlinedButton(
                        onPressed: enabled
                            ? () async {
                                final file = await ImagePicker().pickImage(
                                  source: ImageSource.gallery,
                                  imageQuality: 90,
                                );
                                if (file != null) await onPick(slot, file);
                              }
                            : null,
                        child: Text(
                          syncText(
                            context,
                            '上传／替换证书图片',
                            'Upload / replace certificate',
                          ),
                        ),
                      ),
                      NativeFieldLabel(
                        label: syncText(
                          context,
                          '选择已有图片',
                          'Choose an existing image',
                        ),
                        child: DropdownButtonFormField<int>(
                          key: ValueKey('$slot:${row['media_id']}'),
                          initialValue: row['media_id'] ?? 0,
                          isExpanded: true,
                          decoration: InputDecoration(),
                          items: [
                            const DropdownMenuItem(value: 0, child: Text('—')),
                            for (final id in <int>{
                              ...imageIDs(data),
                              ...credentials.map(
                                (r) => (r['media_id'] ?? 0) as int,
                              ),
                            }.where((id) => id > 0))
                              DropdownMenuItem(value: id, child: Text('#$id')),
                          ],
                          onChanged: enabled
                              ? (id) {
                                  onDiscard(slot);
                                  change(i, {'media_id': id});
                                }
                              : null,
                        ),
                      ),
                      if (local != null)
                        Wrap(
                          children: [
                            TextButton(
                              onPressed: enabled ? onRetry : null,
                              child: Text(m.t('retry')),
                            ),
                            TextButton(
                              onPressed: enabled ? () => onDiscard(slot) : null,
                              child: Text(m.t('remove')),
                            ),
                          ],
                        ),
                      fields(
                        row,
                        slot,
                        (patch) => change(i, patch),
                        requiredTitle: true,
                        issuePrefix: 'company_profile.credentials.$i',
                      ),
                      ordering(i, credentials.length, (delta) {
                        final next = [...credentials], row = next.removeAt(i);
                        next.insert(i + delta, row);
                        update('credentials', next);
                      }),
                      TextButton(
                        onPressed: enabled
                            ? () {
                                onDiscard(slot);
                                final next = [...credentials]..removeAt(i);
                                update('credentials', next);
                              }
                            : null,
                        child: Text(m.t('remove')),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
        OutlinedButton(
          onPressed: enabled && credentials.length < 20
              ? () => update('credentials', [
                  ...credentials,
                  {
                    '_key': '${DateTime.now().microsecondsSinceEpoch}',
                    'kind': 'qualification',
                    'media_id': 0,
                    'title': '',
                  },
                ])
              : null,
          child: Text(
            syncText(
              context,
              '新增资质／荣誉 (${credentials.length}/20)',
              'Add credential (${credentials.length}/20)',
            ),
          ),
        ),
        for (var i = 0; i < gallery.length; i++)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                children: [
                  Photo(id: gallery[i]['media_id'], height: 140),
                  fields(gallery[i], 'gallery:${gallery[i]['media_id']}', (
                    patch,
                  ) {
                    final next = gallery
                        .map(
                          (r) => r['media_id'] == gallery[i]['media_id']
                              ? {...r, ...patch}
                              : r,
                        )
                        .toList();
                    update('gallery_annotations', next);
                  }),
                  ordering(i, gallery.length, (delta) {
                    final ids = imageIDs(data), id = ids.removeAt(i);
                    ids.insert(i + delta, id);
                    onChanged({...data, 'media_ids': ids});
                  }),
                ],
              ),
            ),
          ),
        OutlinedButton(
          onPressed: () => push(
            context,
            PreviewScreen(data: data, kind: 'vendor', photos: photos),
          ),
          child: Text(syncText(context, '预览企业门户', 'Preview company portal')),
        ),
      ],
    );
  }
}
