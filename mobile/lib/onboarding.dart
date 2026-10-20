part of 'main.dart';

class NativeBrandPreview extends StatelessWidget {
  final Json data;
  final List<Json> files;
  const NativeBrandPreview({
    super.key,
    required this.data,
    this.files = const [],
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final p = asMap(data['company_profile'] ?? {});
    Widget image(String slot, double ratio) {
      final local = files
          .where((f) => f['slot'] == slot && f['id'] == null)
          .firstOrNull;
      return local != null
          ? AspectRatio(
              aspectRatio: ratio,
              child: Image.file(
                File(local['path']),
                fit: slot == 'logo_media_id' ? BoxFit.contain : BoxFit.cover,
              ),
            )
          : Photo(
              id: p[slot],
              thumb: false,
              aspectRatio: ratio,
              fit: slot == 'logo_media_id' ? BoxFit.contain : BoxFit.cover,
            );
    }

    final hasCover =
        (p['cover_media_id'] ?? 0) > 0 ||
        files.any((f) => f['slot'] == 'cover_media_id');
    final hasLogo =
        (p['logo_media_id'] ?? 0) > 0 ||
        files.any((f) => f['slot'] == 'logo_media_id');
    return Container(
      margin: const EdgeInsets.only(bottom: 24),
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: industrialNavy,
        borderRadius: BorderRadius.circular(14),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (hasCover) ...[
            ClipRRect(
              borderRadius: BorderRadius.circular(10),
              child: image('cover_media_id', 16 / 9),
            ),
            space(16),
          ],
          if (hasLogo)
            SizedBox(
              width: 64,
              child: ColoredBox(
                color: Colors.white,
                child: image('logo_media_id', 1),
              ),
            ),
          space(12),
          Text(
            m.content(data, 'name'),
            style: const TextStyle(
              color: Colors.white,
              fontSize: 24,
              fontWeight: FontWeight.bold,
            ),
          ),
          space(8),
          Text(
            (data['main_products'] ?? '').toString(),
            style: const TextStyle(color: Color(0xffadc5e0)),
          ),
        ],
      ),
    );
  }
}

String fieldIssueText(AppModel m, Json issue) {
  final zh = m.language == 'zh';
  if (issue['code'] == 'too_long') {
    return zh
        ? '最多${issue['limit']} ${issue['unit'] == 'bytes' ? '字节' : '字'}'
        : 'Maximum ${issue['limit']} ${issue['unit']}';
  }
  return {
        'required': zh ? '请填写此项' : 'Required',
        'contact_required': zh ? '电话或微信至少填写一项' : 'Provide phone or WeChat',
        'invalid_url': zh
            ? '网址需包含 http:// 或 https://'
            : 'Include http:// or https://',
        'category_or_keywords': zh
            ? '请选择分类或填写关键词'
            : 'Choose a category or enter keywords',
        'already_used': zh
            ? '此登录账号已使用，请更换'
            : 'This account name is already used',
        'category_unavailable': zh
            ? '此分类不可选，请更换'
            : 'Choose an available category',
      }[issue['code']] ??
      (zh ? '请检查此项' : 'Check this field');
}

dynamic formValue(Json data, String path) {
  dynamic value = data;
  for (final key in path.split('.')) {
    if (value is List) {
      final index = int.tryParse(key);
      if (index == null || index >= value.length) return null;
      value = value[index];
    } else if (value is Map) {
      value = value[key];
    } else {
      return null;
    }
  }
  return value;
}

Map<String, Json> validateFormData(
  Json data,
  String kind, {
  Set<String> pendingCredentials = const {},
}) {
  final issues = <String, Json>{};
  void add(String field, String code, {int? limit, String? unit}) =>
      issues[field] = {
        'field': field,
        'code': code,
        'limit': limit,
        'unit': unit,
      };
  for (final entry in formRules.entries) {
    if (entry.key.contains('*')) continue;
    final r = asMap(entry.value);
    final value = (formValue(data, entry.key) ?? '').toString();
    if ((r['required'] as List).contains(kind) &&
        value.trim().isEmpty &&
        !(entry.key == 'region_name' && (data['region_id'] ?? 0) > 0)) {
      add(entry.key, 'required');
    }
    final count = r['unit'] == 'bytes'
        ? utf8.encode(value).length
        : value.runes.length;
    if (count > r['max']) {
      add(entry.key, 'too_long', limit: r['max'], unit: r['unit']);
    }
    if (entry.key == 'company_profile.website' && value.isNotEmpty) {
      final u = Uri.tryParse(value);
      if (u == null ||
          !['http', 'https'].contains(u.scheme) ||
          u.host.isEmpty ||
          u.userInfo.isNotEmpty) {
        add(entry.key, 'invalid_url');
      }
    }
  }
  if (kind == 'vendor' &&
      (data['phone'] ?? '').toString().trim().isEmpty &&
      (data['wechat'] ?? '').toString().trim().isEmpty) {
    add('phone', 'contact_required');
    add('wechat', 'contact_required');
  }
  final p = asMap(data['company_profile'] ?? {});
  final contacts = rows(p['contacts']);
  for (var i = 0; i < contacts.length; i++) {
    for (final entry in contacts[i].entries) {
      final r = formRules['company_profile.contacts.*.${entry.key}'];
      if (r == null) continue;
      final value = (entry.value ?? '').toString();
      if ((r['unit'] == 'bytes'
              ? utf8.encode(value).length
              : value.runes.length) >
          r['max']) {
        add(
          'company_profile.contacts.$i.${entry.key}',
          'too_long',
          limit: r['max'],
          unit: r['unit'],
        );
      }
    }
    if ((contacts[i]['phone'] ?? '').toString().trim().isEmpty &&
        (contacts[i]['wechat'] ?? '').toString().trim().isEmpty) {
      add('company_profile.contacts.$i.phone', 'contact_required');
      add('company_profile.contacts.$i.wechat', 'contact_required');
    }
  }
  if (kind == 'vendor') {
    final credentials = rows(p['credentials']);
    if (credentials.length > 20) {
      add('company_profile.credentials', 'too_long', limit: 20, unit: 'items');
    }
    for (var i = 0; i < credentials.length; i++) {
      final r = credentials[i];
      if (!['qualification', 'award'].contains(r['kind'])) {
        add('company_profile.credentials.$i.kind', 'required');
      }
      if ((r['media_id'] ?? 0) == 0 &&
          !pendingCredentials.contains('credential:${r['_key']}')) {
        add('company_profile.credentials.$i.media_id', 'required');
      }
      if ((r['title'] ?? '').toString().trim().isEmpty) {
        add('company_profile.credentials.$i.title', 'required');
      }
    }
    for (final collection in ['credentials', 'gallery_annotations']) {
      final list = rows(p[collection]);
      for (var i = 0; i < list.length; i++) {
        for (final key in [
          'title',
          'title_en',
          'description',
          'description_en',
        ]) {
          final limit = key.startsWith('title') ? 100 : 500;
          if ((list[i][key] ?? '').toString().runes.length > limit) {
            add(
              'company_profile.$collection.$i.$key',
              'too_long',
              limit: limit,
              unit: 'characters',
            );
          }
        }
      }
    }
  }
  return issues;
}

InputDecoration guidedDecoration(
  AppModel m,
  String path,
  String value,
  String label, {
  Json? issue,
}) {
  final r = formRules[path.replaceAll(RegExp(r'\.\d+\.'), '.*.')];
  if (r == null) {
    return InputDecoration(
      labelText: label,
      errorText: issue == null ? null : fieldIssueText(m, issue),
    );
  }
  final count = r['unit'] == 'bytes'
      ? utf8.encode(value).length
      : value.runes.length;
  return InputDecoration(
    labelText: label,
    hintText: r[m.language == 'zh' ? 'sample_zh' : 'sample_en'],
    helperText:
        '${r[m.language]} · $count/${r['max']} ${m.language == 'zh' ? (r['unit'] == 'bytes' ? '字节' : '字') : r['unit']}\n${m.language == 'zh' ? '例：' : 'e.g. '}${r[m.language == 'zh' ? 'sample_zh' : 'sample_en']}',
    helperMaxLines: 6,
    errorText: issue == null ? null : fieldIssueText(m, issue),
    errorMaxLines: 3,
  );
}

class NativeJoinGuide extends StatelessWidget {
  const NativeJoinGuide({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Container(
      padding: const EdgeInsets.all(16),
      margin: const EdgeInsets.only(bottom: 20),
      decoration: BoxDecoration(
        color: const Color(0xffeaf2ff),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        m.language == 'zh'
            ? '注册账号 → 完善企业资料 → 提交审核 → 公开展示\n本机草稿仅在此设备；云端保存后可在网站和 App 继续编辑，审核通过后才对外展示。'
            : 'Create account → Complete profile → Submit → Public showroom\nLocal drafts stay on this device. Cloud drafts work on web and app; review is required for publication.',
        style: const TextStyle(fontSize: 13, height: 1.8),
      ),
    );
  }
}

class NativeCategoryPicker extends StatefulWidget {
  final int value;
  final String kind;
  final int? hallID;
  final bool enabled;
  final ValueChanged<int> onChanged;
  final Json? issue;
  const NativeCategoryPicker({
    super.key,
    required this.value,
    this.kind = 'vendor',
    this.hallID,
    required this.enabled,
    required this.onChanged,
    this.issue,
  });
  @override
  State<NativeCategoryPicker> createState() => _NativeCategoryPickerState();
}

class _NativeCategoryPickerState extends State<NativeCategoryPicker> {
  int version = 0;
  Future<void> suggest(AppModel m, List<Json> cats) async {
    if (m.token.isEmpty) {
      final ok = await Navigator.push<bool>(
        context,
        MaterialPageRoute(
          builder: (_) => LoginScreen(register: m.account.isEmpty),
        ),
      );
      if (ok != true || !mounted) return;
      setState(() => version++);
      cats = rows((await m.api('/merchant/taxonomy'))['categories']);
      if (!mounted) return;
    }
    final formal = cats
        .where(
          (c) =>
              c['enabled'] == true &&
              ((c['suggestion_status'] ?? '') == '' ||
                  c['suggestion_status'] == 'approved'),
        )
        .toList();
    final name = TextEditingController(), en = TextEditingController();
    var parent = formal.any((c) => c['id'] == widget.value) ? widget.value : 0;
    var error = '';
    var busy = false;
    final dialog = DialogRoute<void>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setDialog) => AlertDialog(
          title: Text(m.language == 'zh' ? '补充分类' : 'Suggest category'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  m.language == 'zh'
                      ? '先供本厂商选用，管理员以后统一规范，不影响内容审核。'
                      : 'Initially available to your company; standardized later.',
                ),
                space(),
                TextField(
                  controller: name,
                  decoration: InputDecoration(
                    labelText: m.language == 'zh'
                        ? '中文分类名称'
                        : 'Chinese category name',
                    hintText: '液压翻转犁',
                    helperText: '最多300字节 / up to 300 bytes',
                  ),
                ),
                space(),
                TextField(
                  controller: en,
                  decoration: InputDecoration(
                    labelText: m.t('nameEN'),
                    hintText: 'Hydraulic reversible plough',
                  ),
                ),
                space(),
                DropdownButtonFormField<int>(
                  initialValue: parent,
                  isExpanded: true,
                  decoration: InputDecoration(labelText: m.t('parent')),
                  items: [
                    DropdownMenuItem(
                      value: 0,
                      child: Text(
                        syncText(context, '顶级（一级）分类', 'Top-level category'),
                      ),
                    ),
                    ...formal.map(
                      (c) => DropdownMenuItem<int>(
                        value: c['id'],
                        child: Text(m.content(c, 'name')),
                      ),
                    ),
                  ],
                  onChanged: (v) => parent = v ?? 0,
                ),
                if (error.isNotEmpty)
                  Text(error, style: const TextStyle(color: Colors.red)),
              ],
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
                          utf8.encode(name.text).length > 300) {
                        setDialog(
                          () => error = m.language == 'zh'
                              ? '填写名称，最多300字节'
                              : 'Enter a name, up to 300 bytes',
                        );
                        return;
                      }
                      setDialog(() => busy = true);
                      try {
                        final cat = asMap(
                          await m.api(
                            '/merchant/categories',
                            method: 'POST',
                            data: {
                              'name': name.text,
                              'name_en': en.text,
                              'parent_id': parent,
                              'hall_id': widget.hallID ?? m.activeHall.id,
                            },
                          ),
                        );
                        if (!context.mounted) return;
                        widget.onChanged(cat['id']);
                        setState(() => version++);
                        Navigator.pop(context);
                      } catch (e) {
                        setDialog(
                          () => error = e is ApiFailure
                              ? m.t(e.code)
                              : m.t('network_error'),
                        );
                      } finally {
                        if (context.mounted) setDialog(() => busy = false);
                      }
                    },
              child: Text(m.t('save')),
            ),
          ],
        ),
      ),
    );
    await Navigator.of(context).push(dialog);
    await dialog.completed;
    name.dispose();
    en.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return AsyncPane(
      identity:
          '${m.baseUrl}:category-picker:${m.scope}:${widget.hallID ?? m.activeHall.id}:${m.token.isNotEmpty}:$version',
      load: () {
        final hall = m.availableHalls
            .where((h) => h.id == (widget.hallID ?? m.activeHall.id))
            .firstOrNull;
        return m.api(
          (m.token.isEmpty ? '/public/taxonomy' : '/merchant/taxonomy') +
              (hall == null
                  ? ''
                  : '?hall=${Uri.encodeQueryComponent(hall.slug)}'),
        );
      },
      builder: (r) {
        final cats = rows(r['categories'])
            .where(
              (cat) =>
                  cat['hall_id'] == null ||
                  cat['hall_id'] == (widget.hallID ?? m.activeHall.id),
            )
            .toList();
        final selected =
            rows(r['aliases'])
                .where((a) => a['source_id'] == widget.value)
                .firstOrNull?['target_id'] ??
            widget.value;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            DropdownButtonFormField<int>(
              key: ValueKey('${widget.value}:${cats.length}'),
              initialValue: selected,
              isExpanded: true,
              decoration: InputDecoration(
                labelText: m.t('category'),
                errorText: widget.issue == null
                    ? null
                    : fieldIssueText(m, widget.issue!),
              ),
              items: [
                DropdownMenuItem(value: 0, child: Text(m.t('optional'))),
                ...cats.map(
                  (c) => DropdownMenuItem<int>(
                    value: c['id'],
                    enabled: c['enabled'] == true || c['id'] == widget.value,
                    child: Text(
                      '${m.content(c, 'name')}${c['suggestion_status'] == 'pending' ? (m.language == 'zh' ? '（待管理员规范）' : ' (pending)') : ''}',
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ),
                if (selected > 0 && !cats.any((c) => c['id'] == selected))
                  DropdownMenuItem(value: selected, child: Text('#$selected')),
              ],
              onChanged: widget.enabled
                  ? (v) => widget.onChanged(v ?? 0)
                  : null,
            ),
            TextButton(
              onPressed: widget.enabled ? () => suggest(m, cats) : null,
              child: Text(
                m.language == 'zh'
                    ? (widget.kind == 'vendor' ? '＋ 新增经营分类' : '＋ 新增产品分类')
                    : (widget.kind == 'vendor'
                          ? 'Add business category'
                          : 'Add product category'),
              ),
            ),
          ],
        );
      },
    );
  }
}

class NativeBrandImageEditor extends StatelessWidget {
  final String slot;
  final int value;
  final List<int> ids;
  final Json? pending;
  final bool enabled;
  final int progress;
  final ValueChanged<int> onChanged;
  final Future<void> Function(XFile) onPick;
  final VoidCallback onRetry;
  final VoidCallback? onDiscard;
  const NativeBrandImageEditor({
    super.key,
    required this.slot,
    required this.value,
    required this.ids,
    required this.enabled,
    required this.onChanged,
    required this.onPick,
    required this.onRetry,
    this.onDiscard,
    this.pending,
    this.progress = 0,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final logo = slot == 'logo_media_id';
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              m.t(logo ? 'logo' : 'cover'),
              style: Theme.of(context).textTheme.titleMedium,
            ),
            space(),
            Text(
              m.language == 'zh'
                  ? 'JPEG、PNG、WebP，最多10 MiB；建议${logo ? '方形' : '16:9'}，不强制裁剪。'
                  : 'JPEG, PNG or WebP, up to 10 MiB; ${logo ? 'square' : '16:9'} recommended.',
            ),
            space(),
            AspectRatio(
              aspectRatio: logo ? 1 : 16 / 9,
              child: pending != null
                  ? Image.file(
                      File(pending!['path']),
                      fit: logo ? BoxFit.contain : BoxFit.cover,
                      errorBuilder: (_, _, _) =>
                          const Icon(Icons.image_not_supported_outlined),
                    )
                  : value > 0
                  ? Photo(
                      id: value,
                      thumb: false,
                      aspectRatio: logo ? 1 : 16 / 9,
                      fit: logo ? BoxFit.contain : BoxFit.cover,
                    )
                  : const ColoredBox(
                      color: industrialSurface,
                      child: Center(
                        child: Icon(
                          Icons.add_photo_alternate_outlined,
                          size: 36,
                          color: Color(0xff8ba0ba),
                        ),
                      ),
                    ),
            ),
            space(),
            Wrap(
              spacing: 12,
              runSpacing: 8,
              children: [
                OutlinedButton.icon(
                  onPressed: enabled
                      ? () async {
                          final file = await ImagePicker().pickImage(
                            source: ImageSource.gallery,
                          );
                          if (file != null) await onPick(file);
                        }
                      : null,
                  icon: const Icon(Icons.upload_outlined),
                  label: Text(m.t('upload')),
                ),
                if (value > 0 || pending != null)
                  TextButton(
                    onPressed: enabled
                        ? () {
                            onChanged(0);
                            onDiscard?.call();
                          }
                        : null,
                    child: Text(m.t('remove')),
                  ),
                if (pending != null)
                  TextButton(
                    onPressed: enabled ? onRetry : null,
                    child: Text(m.t('retry')),
                  ),
              ],
            ),
            if (!enabled && pending != null) ...[
              LinearProgressIndicator(value: progress / 100),
              Text('$progress%'),
            ],
            if (ids.isNotEmpty)
              ExpansionTile(
                title: Text(
                  m.language == 'zh' ? '从已有图片选择' : 'Choose existing image',
                ),
                children: [
                  Wrap(
                    spacing: 8,
                    runSpacing: 8,
                    children: ids
                        .map(
                          (id) => SizedBox(
                            width: 64,
                            height: 64,
                            child: InkWell(
                              onTap: enabled ? () => onChanged(id) : null,
                              child: Photo(id: id, aspectRatio: 1),
                            ),
                          ),
                        )
                        .toList(),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }
}
