part of 'main.dart';

class PromotionalProfileEditor extends StatelessWidget {
  final String? initialSection;
  final Map<String, GlobalKey>? sectionKeys;
  final Json data;
  final ValueChanged<Json> onChanged;
  final bool enabled;
  final Map<String, Json> issues;
  final Map<String, GlobalKey>? fieldKeys;
  final List<Json> brandFiles;
  final int brandProgress;
  final Future<void> Function(String, XFile)? onBrandPick;
  final Future<void> Function()? onBrandRetry;
  final ValueChanged<String>? onBrandDiscard;
  const PromotionalProfileEditor({
    super.key,
    this.initialSection,
    this.sectionKeys,
    required this.data,
    required this.onChanged,
    required this.enabled,
    this.issues = const {},
    this.fieldKeys,
    this.brandFiles = const [],
    this.brandProgress = 0,
    this.onBrandPick,
    this.onBrandRetry,
    this.onBrandDiscard,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final p = asMap(data['company_profile'] ?? {});
    void update(String key, dynamic value) {
      onChanged({
        ...data,
        'company_profile': {...p, key: value},
      });
    }

    Widget field(String key, String label, {Json? contact, int? index}) {
      return Padding(
        key: fieldKeys?.putIfAbsent(
          contact == null
              ? 'company_profile.$key'
              : 'company_profile.contacts.$index.$key',
          () => GlobalKey(),
        ),
        padding: const EdgeInsets.only(bottom: 16),
        child: NativeFieldLabel(
          label: m.t(label),
          child: TextFormField(
            key: ValueKey('${index ?? 'profile'}:$key'),
            initialValue:
                (contact == null ? p[key] : contact[key])?.toString() ?? '',
            enabled: enabled,
            decoration: guidedDecoration(
              m,
              contact == null
                  ? 'company_profile.$key'
                  : 'company_profile.contacts.$index.$key',
              (contact == null ? p[key] : contact[key])?.toString() ?? '',
              m.t(label),
              issue:
                  issues[contact == null
                      ? 'company_profile.$key'
                      : 'company_profile.contacts.$index.$key'],
            ),
            onChanged: (v) {
              if (contact == null) {
                update(key, v);
              } else {
                final cs = rows(p['contacts']);
                cs[index!] = {...contact, key: v};
                update('contacts', cs);
              }
            },
          ),
        ),
      );
    }

    const fields = {
      'short_name': 'shortName',
      'summary': 'summary',
      'established': 'established',
      'staff': 'staff',
      'factory_area': 'factory_area',
      'equipment': 'equipment',
      'capacity': 'capacity',
      'website': 'website',
      'contact_name': 'contactName',
    };
    const english = {
      'short_name_en': 'shortNameEN',
      'summary_en': 'summaryEN',
      'equipment_en': 'equipmentEN',
      'capacity_en': 'capacityEN',
      'contact_name_en': 'contactNameEN',
    };
    final ids = {
      <int>{
        ...imageIDs(data),
        if (p['logo_media_id'] is int && p['logo_media_id'] > 0)
          p['logo_media_id'],
        if (p['cover_media_id'] is int && p['cover_media_id'] > 0)
          p['cover_media_id'],
      },
    }.first;
    final contacts = rows(p['contacts']);
    return ExpansionTile(
      key: ValueKey(
        'profile-${issues.keys.any((k) => k.startsWith("company_profile"))}',
      ),
      initiallyExpanded:
          ['brand', 'profile'].contains(initialSection) ||
          issues.keys.any((k) => k.startsWith('company_profile')),
      title: Text(m.t('profile')),
      childrenPadding: const EdgeInsets.all(12),
      children: [
        SizedBox(key: sectionKeys?['profile']),
        for (final f in fields.entries) field(f.key, f.value),
        ExpansionTile(
          title: Text(m.t('englishInfo')),
          children: [for (final f in english.entries) field(f.key, f.value)],
        ),
        for (final k in ['logo_media_id', 'cover_media_id'])
          NativeBrandImageEditor(
            key: k == 'logo_media_id' ? (sectionKeys?['brand']) : null,
            slot: k,
            value: p[k] ?? 0,
            ids: ids.toList(),
            pending: brandFiles.where((f) => f['slot'] == k).firstOrNull,
            enabled: enabled,
            progress: brandProgress,
            onChanged: (id) => update(k, id),
            onPick: (file) async {
              if (onBrandPick != null) await onBrandPick!(k, file);
            },
            onRetry: () => onBrandRetry?.call(),
            onDiscard: () => onBrandDiscard?.call(k),
          ),
        Text(m.t('regionalContacts')),
        for (var i = 0; i < contacts.length; i++)
          Card(
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                children: [
                  field('area', 'serviceArea', contact: contacts[i], index: i),
                  field(
                    'name',
                    'contactPerson',
                    contact: contacts[i],
                    index: i,
                  ),
                  field('phone', 'phone', contact: contacts[i], index: i),
                  field('wechat', 'wechat', contact: contacts[i], index: i),
                  ExpansionTile(
                    title: Text(m.t('englishInfo')),
                    children: [
                      field(
                        'area_en',
                        'serviceAreaEN',
                        contact: contacts[i],
                        index: i,
                      ),
                      field(
                        'name_en',
                        'contactPersonEN',
                        contact: contacts[i],
                        index: i,
                      ),
                    ],
                  ),
                  TextButton(
                    onPressed: enabled
                        ? () {
                            contacts.removeAt(i);
                            update('contacts', contacts);
                          }
                        : null,
                    child: Text(m.t('remove')),
                  ),
                ],
              ),
            ),
          ),
        OutlinedButton(
          onPressed: enabled && contacts.length < 20
              ? () => update('contacts', [
                  ...contacts,
                  {'area': '', 'name': '', 'phone': '', 'wechat': ''},
                ])
              : null,
          child: Text(m.t('addContact')),
        ),
      ],
    );
  }
}

String showroomShareURL(String canonical, String base) {
  if (RegExp(r'[\\\x00-\x20]').hasMatch(canonical)) {
    throw ApiFailure('invalid_request');
  }
  final resolved = Uri.tryParse(base)?.resolve(canonical);
  if (resolved == null ||
      !['http', 'https'].contains(resolved.scheme) ||
      resolved.host.isEmpty ||
      resolved.userInfo.isNotEmpty) {
    throw ApiFailure('invalid_request');
  }
  return resolved.toString().split('?').first.split('#').first;
}

String showroomAddressScope(String url) {
  final h = Uri.parse(url).host;
  if (['localhost', '127.0.0.1', '::1'].contains(h)) return 'localOnly';
  if (RegExp(
        r'^(10\.|192\.168\.|172\.(1[6-9]|2\d|3[01])\.|169\.254\.)',
      ).hasMatch(h) ||
      h.startsWith('fd') ||
      h.startsWith('fe80:')) {
    return 'lanShare';
  }
  return '';
}

const sharingBridge = MethodChannel('com.zqzt.dalunongji/sharing');
const domesticShareKeys = [
  'wechatFriend',
  'wechatMoments',
  'qq',
  'qzone',
  'wecom',
  'dingtalk',
];
const overseasShareKeys = ['whatsapp', 'telegram', 'email'];

class ShowroomPromotion extends StatefulWidget {
  final String slug;
  const ShowroomPromotion({super.key, required this.slug});
  @override
  State<ShowroomPromotion> createState() => _ShowroomPromotionState();
}

class _ShowroomPromotionState extends State<ShowroomPromotion> {
  String channel = 'wechatFriend';
  Uint8List? preview;
  bool busy = false;
  int revision = 0;
  Future<void> copy(String value) async {
    try {
      await Clipboard.setData(ClipboardData(text: value));
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(ModelScope.of(context).t('copied'))),
        );
      }
    } catch (e) {
      if (mounted) showError(context, ApiFailure('copyFallback'));
    }
  }

  Future<void> image(
    AppModel m,
    Json item,
    String url,
    bool poster,
    bool save,
  ) async {
    setState(() => busy = true);
    try {
      final bytes = await showroomPNG(m, item, url, poster: poster);
      if (!mounted) return;
      setState(() => preview = bytes);
      await sharingBridge.invokeMethod(save ? 'saveImage' : 'shareImage', {
        'bytes': bytes,
        'name': 'showroom-${item['id']}-${poster ? 'poster' : 'qr'}.png',
        'title': m.content(asMap(item['data']), 'name'),
      });
    } catch (e) {
      if (mounted) showError(context, ApiFailure('posterFailed'));
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Theme(
      data: merchantTheme(context),
      child: Scaffold(
        appBar: appBar(context, m.t('promotion')),
        body: SafeArea(
          top: false,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              AsyncPane(
                key: ValueKey('${m.scope}:${m.token.hashCode}:${widget.slug}'),
                identity:
                    '${m.scope}:${m.token.hashCode}:${widget.slug}:$revision',
                load: () => m.api(
                  '/public/showrooms/${Uri.encodeComponent(widget.slug)}',
                ),
                builder: (r) {
                  final item = asMap(r['item']),
                      data = asMap(item['data']),
                      profile = asMap(data['company_profile'] ?? {});
                  final url = showroomShareURL(r['canonical_url'], m.baseUrl);
                  final title = m.content(data, 'name');
                  final summary = m.content(profile, 'summary').isNotEmpty
                      ? m.content(profile, 'summary')
                      : m.content(data, 'description');
                  final text = [
                    title,
                    if (summary.isNotEmpty) summary,
                    url,
                  ].join('\n');
                  final cover = [
                    profile['logo_media_id'],
                    profile['cover_media_id'],
                    ...imageIDs(data),
                  ].whereType<num>().where((id) => id > 0).firstOrNull;
                  final uri = Uri.parse(url);
                  final scope = showroomAddressScope(url);
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Card(
                        child: Padding(
                          padding: const EdgeInsets.all(12),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              if (cover is num && cover > 0) ...[
                                SizedBox(
                                  width: 72,
                                  height: 72,
                                  child: Photo(id: cover.toInt(), height: 72),
                                ),
                                const SizedBox(width: 12),
                              ],
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      title,
                                      maxLines: 2,
                                      overflow: TextOverflow.ellipsis,
                                      style: const TextStyle(
                                        fontSize: 18,
                                        fontWeight: FontWeight.w600,
                                      ),
                                    ),
                                    if (summary.isNotEmpty)
                                      Text(
                                        summary,
                                        maxLines: 3,
                                        overflow: TextOverflow.ellipsis,
                                        style: const TextStyle(fontSize: 14),
                                      ),
                                  ],
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                      if (scope.isNotEmpty ||
                          uri.scheme != 'https' ||
                          RegExp(r'^\d{1,3}(\.\d{1,3}){3}$').hasMatch(uri.host))
                        message(
                          context,
                          m.t(scope.isNotEmpty ? scope : 'testShareAddress'),
                        ),
                      Padding(
                        padding: const EdgeInsets.symmetric(vertical: 12),
                        child: SelectableText(
                          url,
                          style: const TextStyle(fontSize: 13),
                        ),
                      ),
                      FilledButton.icon(
                        onPressed: busy
                            ? null
                            : () => sharePublicText(context, text, title, url),
                        icon: const Icon(Icons.share_outlined),
                        label: Text(m.t('shareShowroom')),
                      ),
                      space(8),
                      Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: [
                          OutlinedButton.icon(
                            onPressed: () => copy(url),
                            icon: const Icon(Icons.link, size: 18),
                            label: Text(m.t('copyLink')),
                          ),
                          OutlinedButton.icon(
                            onPressed: busy
                                ? null
                                : () async {
                                    setState(() => busy = true);
                                    try {
                                      final bytes = await showroomPNG(
                                        m,
                                        item,
                                        url,
                                        poster: false,
                                      );
                                      if (!context.mounted) return;
                                      await showDialog<void>(
                                        context: context,
                                        builder: (c) => AlertDialog(
                                          title: Text(m.t('showroomQR')),
                                          content: SizedBox(
                                            width: 260,
                                            child: Image.memory(bytes),
                                          ),
                                          actions: [
                                            TextButton(
                                              onPressed: () => Navigator.pop(c),
                                              child: Text(m.t('close')),
                                            ),
                                            TextButton(
                                              onPressed: () async {
                                                try {
                                                  await sharingBridge.invokeMethod(
                                                    'saveImage',
                                                    {
                                                      'bytes': bytes,
                                                      'name':
                                                          'showroom-${item['id']}-qr.png',
                                                    },
                                                  );
                                                } catch (_) {
                                                  if (c.mounted) {
                                                    showError(
                                                      c,
                                                      ApiFailure(
                                                        'posterFailed',
                                                      ),
                                                    );
                                                  }
                                                }
                                              },
                                              child: Text(m.t('downloadQR')),
                                            ),
                                          ],
                                        ),
                                      );
                                    } catch (_) {
                                      if (context.mounted) {
                                        showError(
                                          context,
                                          ApiFailure('posterFailed'),
                                        );
                                      }
                                    } finally {
                                      if (mounted) setState(() => busy = false);
                                    }
                                  },
                            icon: const Icon(Icons.qr_code, size: 18),
                            label: Text(m.t('showroomQR')),
                          ),
                          OutlinedButton.icon(
                            onPressed: busy
                                ? null
                                : () => image(m, item, url, true, true),
                            icon: const Icon(Icons.download_outlined, size: 18),
                            label: Text(m.t('downloadPoster')),
                          ),
                        ],
                      ),
                      if (busy)
                        const Padding(
                          padding: EdgeInsets.all(16),
                          child: Center(child: CircularProgressIndicator()),
                        ),
                      if (preview != null)
                        Padding(
                          padding: const EdgeInsets.only(top: 16),
                          child: Image.memory(preview!),
                        ),
                      ExpansionTile(
                        tilePadding: EdgeInsets.zero,
                        title: Text(
                          m.t('shareMoreHelp'),
                          style: const TextStyle(fontSize: 14),
                        ),
                        children: [
                          SelectableText(text),
                          space(8),
                          Text(m.t('shareGuide')),
                          TextButton(
                            onPressed: () => copy(text),
                            child: Text(m.t('copyPromotion')),
                          ),
                          TextButton(
                            onPressed: busy
                                ? null
                                : () => image(m, item, url, true, false),
                            child: Text(m.t('sharePoster')),
                          ),
                        ],
                      ),
                    ],
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }
}

Future<Uint8List> showroomPNG(
  AppModel m,
  Json item,
  String url, {
  required bool poster,
}) async {
  final d = asMap(item['data']);
  final p = asMap(d['company_profile'] ?? {});
  final rec = ui.PictureRecorder();
  final canvas = Canvas(rec);
  const width = 900;
  final name = m.content(d, 'name');
  TextPainter text(
    String value,
    double size, {
    bool bold = false,
    Color color = green,
    double maxWidth = 772,
  }) => TextPainter(
    text: TextSpan(
      text: value,
      style: TextStyle(
        fontSize: size,
        fontFamily: 'sans-serif',
        fontWeight: bold ? FontWeight.bold : FontWeight.normal,
        color: color,
      ),
    ),
    textDirection: TextDirection.ltr,
  )..layout(maxWidth: maxWidth);
  final title = text(name, 46, bold: true);
  final lead = text((d['main_products'] ?? '').toString(), 26);
  final height = poster ? (1200 + title.height + lead.height).ceil() : 500;
  canvas.drawRect(
    Rect.fromLTWH(0, 0, width.toDouble(), height.toDouble()),
    Paint()..color = Colors.white,
  );
  void qr(Rect box) {
    final q = QrImage(
      QrCode(
        payload: QrPayload.fromString(url),
        errorCorrectLevel: QrErrorCorrectLevel.medium,
      ),
    );
    final step = (box.width / (q.moduleCount + 8)).floorToDouble();
    final left = (box.left + (box.width - step * q.moduleCount) / 2)
        .roundToDouble();
    final top = (box.top + (box.height - step * q.moduleCount) / 2)
        .roundToDouble();
    for (var y = 0; y < q.moduleCount; y++) {
      for (var x = 0; x < q.moduleCount; x++) {
        if (q.isDark(y, x)) {
          canvas.drawRect(
            Rect.fromLTWH(left + x * step, top + y * step, step, step),
            Paint()
              ..color = Colors.black
              ..isAntiAlias = false,
          );
        }
      }
    }
  }

  if (!poster) {
    qr(const Rect.fromLTWH(200, 0, 500, 500));
  } else {
    canvas.drawRect(const Rect.fromLTWH(0, 0, 900, 20), Paint()..color = green);
    text(m.t('showroom'), 24).paint(canvas, const Offset(64, 70));
    var y = 135.0;
    title.paint(canvas, Offset(64, y));
    y += title.height + 20;
    lead.paint(canvas, Offset(64, y));
    y += lead.height + 30;
    final box = Rect.fromLTWH(64, y, 772, 390);
    canvas.drawRect(box, Paint()..color = pale);
    final cover = [
      p['cover_media_id'],
      ...imageIDs(d),
      p['logo_media_id'],
    ].whereType<num>().where((id) => id > 0).firstOrNull?.toInt();
    bool loaded = false;
    if (cover != null && cover != 0) {
      try {
        final r = await http
            .get(Uri.parse('${m.baseUrl}/api/v1/media/$cover'))
            .timeout(const Duration(seconds: 15));
        if (r.statusCode == 200) {
          final codec = await ui.instantiateImageCodec(r.bodyBytes);
          final frame = await codec.getNextFrame();
          paintImage(
            canvas: canvas,
            rect: box,
            image: frame.image,
            fit: BoxFit.contain,
          );
          frame.image.dispose();
          codec.dispose();
          loaded = true;
        }
      } catch (_) {}
    }
    if (!loaded) {
      text(m.t('showroom'), 36, bold: true).paint(canvas, Offset(100, y + 165));
    }
    y += 430;
    text((d['region_name'] ?? '').toString(), 24).paint(canvas, Offset(64, y));
    y += 48;
    text(
      (d['phone'] ?? '').toString(),
      32,
      bold: true,
    ).paint(canvas, Offset(64, y));
    qr(Rect.fromLTWH(64, height - 355.0, 280, 280));
    text(
      m.t('posterCaption'),
      25,
      maxWidth: 440,
    ).paint(canvas, Offset(380, height - 265.0));
  }
  final picture = rec.endRecording();
  final raster = await picture.toImage(width, height);
  final bytes = await raster.toByteData(format: ui.ImageByteFormat.png);
  raster.dispose();
  picture.dispose();
  if (bytes == null) throw StateError('png_unavailable');
  return bytes.buffer.asUint8List();
}
