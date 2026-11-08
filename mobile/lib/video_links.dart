part of 'main.dart';

String videoV2(String path) =>
    '$path${path.contains('?') ? '&' : '?'}video_api_version=2';
bool isDouyinVideo(Json item) =>
    item['kind'] == 'video' && item['data']?['video_source'] == 'DOUYIN';
bool isKuaishouVideo(Json item) =>
    item['kind'] == 'video' && item['data']?['video_source'] == 'KUAISHOU';
bool isExternalVideo(Json item) => isDouyinVideo(item) || isKuaishouVideo(item);
bool canPreviewVideo(Json? check) =>
    check?['status'] == 'READY' || check?['status'] == 'EXTERNAL_ONLY';

extension VideoCapabilities on AppModel {
  Future<int> videoVersion() async {
    try {
      final caps = await clientCapabilities();
      return (caps['video_api_version'] as num? ?? 2) >= 3 ? 3 : 2;
    } catch (_) {
      return 2;
    }
  }

  Future<String> videoPath(String path) async =>
      '$path${path.contains('?') ? '&' : '?'}video_api_version=${await videoVersion()}';
}

const videoPurposes = {
  'company': 'companyVideo',
  'product': 'productVideo',
  'production': 'productionVideo',
  'other': 'otherVideo',
};

class NativeVideoPurpose extends StatelessWidget {
  final String value;
  final bool filter;
  final ValueChanged<String>? onChanged;
  const NativeVideoPurpose({
    super.key,
    required this.value,
    this.filter = false,
    this.onChanged,
  });

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return NativeFieldLabel(
      label: m.t('purpose'),
      child: FutureBuilder<int>(
        future: m.videoVersion(),
        builder: (_, snapshot) => DropdownButtonFormField<String>(
          key: ValueKey(value),
          initialValue: value,
          isExpanded: true,
          decoration: const InputDecoration(),
          items: [
            if (filter)
              DropdownMenuItem(value: '', child: Text(m.t('allVideoTypes'))),
            for (final entry in videoPurposes.entries)
              if (entry.key != 'other' ||
                  snapshot.data == 3 ||
                  value == 'other')
                DropdownMenuItem(
                  value: entry.key,
                  child: Text(m.t(entry.value)),
                ),
          ],
          onChanged: onChanged == null
              ? null
              : (value) {
                  if (value != null) onChanged!(value);
                },
        ),
      ),
    );
  }
}

Uri? trustedExternalVideo(String raw, {required bool kuaishou}) {
  final uri = Uri.tryParse(raw);
  if (uri == null ||
      uri.scheme != 'https' ||
      uri.hasPort ||
      uri.userInfo.isNotEmpty ||
      uri.hasFragment) {
    return null;
  }
  if (kuaishou) {
    if (uri.hasQuery) return null;
    final path = switch (uri.host) {
      'v.kuaishou.com' => RegExp(r'^/[A-Za-z0-9]{3,48}$'),
      'www.kuaishou.com' => RegExp(r'^/short-video/[A-Za-z0-9]{3,48}$'),
      _ => null,
    };
    return path?.hasMatch(uri.path) == true ? uri : null;
  }
  return const ['www.douyin.com', 'v.douyin.com'].contains(uri.host)
      ? uri
      : null;
}

Uri? trustedDouyinPlayer(String raw, String id) {
  final u = Uri.tryParse(raw);
  return u != null &&
          u.scheme == 'https' &&
          u.host == 'open.douyin.com' &&
          !u.hasPort &&
          u.userInfo.isEmpty &&
          !u.hasFragment &&
          RegExp(r'^[1-9][0-9]{8,23}$').hasMatch(id) &&
          u.path == '/player/video' &&
          u.queryParameters['vid'] == id &&
          u.queryParameters['autoplay'] == '0' &&
          u.queryParametersAll.length == 2 &&
          u.queryParametersAll.values.every((values) => values.length == 1)
      ? u
      : null;
}

String videoErrorText(Object e) {
  final code = e is ApiFailure ? e.code : e.toString();
  return const <String, String>{
        'agreement_required': '请先在下方确认当前版本的企业服务协议，已确认后无需重复操作。',
        'unsupported_video_content': '这是抖音图文链接，当前视频播放器暂不支持。请复制视频作品的分享链接。',
        'video_id_not_found': '暂未识别视频，请使用完整的视频链接重新检查。',
        'video_redirect_limit': '分享链接跳转次数过多，请使用完整视频链接。',
        'video_provider_unavailable': '抖音服务暂时无法连接，请稍后重试。',
        'invalid_official_player': '抖音未返回可用播放器，请换一个公开视频或稍后重试。',
        'video_check_required': '请先检查链接并确认能够预览，再提交审核。',
        'video_rights_required': '请确认您有权展示该视频。',
        'invalid_video_link': '请输入有效的视频分享链接。',
        'one_video_link_required': '请每次粘贴一个视频链接。',
        'duplicate_video': '这个视频已添加过，请到视频列表修改。',
        'video_not_public': '该视频目前无法公开播放，请确认公开视频状态。',
        'video_network_timeout': '连接抖音超时，请稍后重新检查。',
        'video_link_unreachable': '分享短链接暂时无法访问，请稍后重试。',
        'douyin_video_disabled': '抖音链接暂未开放，本地上传照常使用。',
        'video_client_upgrade_required': '请使用新版网站或 App 管理此视频。',
        'forbidden': '您没有操作这个视频的权限。',
      }[code] ??
      '暂时无法播放或保存，请重新检查链接后重试。';
}

bool _officialVideoOpening = false;
Future<void> openOfficialVideo(Json check, {bool Function()? canOpen}) async {
  final id = (check['video_id'] ?? '').toString();
  final url = trustedDouyinPlayer((check['player_url'] ?? '').toString(), id);
  if (check['status'] != 'READY' || url == null) {
    throw ApiFailure((check['reason'] ?? 'video_check_required').toString());
  }
  if (_officialVideoOpening) return;
  _officialVideoOpening = true;
  try {
    await NativeVideoSessions.stopAll();
    if (canOpen != null && !canOpen()) return;
    // Only the validated public player URL and dimensions cross into WebView.
    await const MethodChannel(
      'com.zqzt.dalunongji/video',
    ).invokeMethod('open', {
      'url': url.toString(),
      'width': check['width'] ?? 0,
      'height': check['height'] ?? 0,
    });
  } finally {
    _officialVideoOpening = false;
  }
}

Object? _nativeResourceOpening;
Future<void> openNativeResource(
  BuildContext context,
  Json item, {
  bool private = false,
  Json? check,
  List<int> fallbackIDs = const [],
}) async {
  if (_nativeResourceOpening != null) return;
  final opening = Object();
  _nativeResourceOpening = opening;
  void releaseOpening() {
    if (identical(_nativeResourceOpening, opening)) {
      _nativeResourceOpening = null;
    }
  }

  final origin = ModalRoute.of(context);
  final model = ModelScope.of(context);
  final scope = model.scope, token = model.token, endpoint = model.baseUrl;
  bool active() =>
      context.mounted &&
      model.scope == scope &&
      model.token == token &&
      model.baseUrl == endpoint &&
      (origin == null || origin.isCurrent) &&
      (WidgetsBinding.instance.lifecycleState == null ||
          WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed);
  try {
    if (isExternalVideo(item)) {
      final data = asMap(item['data']);
      final kuaishou = isKuaishouVideo(item);
      String? failure;
      if (!kuaishou) {
        try {
          final result =
              check ??
              asMap(
                await model.api(
                  await model.videoPath(
                    '/${private ? 'merchant' : 'public'}/resources/${item['id']}/playback',
                  ),
                ),
              );
          if (!active()) return;
          await openOfficialVideo(result, canOpen: active);
          return;
        } catch (e) {
          failure = videoErrorText(e);
        }
      }
      await NativeVideoSessions.stopAll();
      if (!context.mounted || !active()) return;
      final uri = trustedExternalVideo(
        (check?['canonical_url'] ?? data['canonical_video_url'] ?? '')
            .toString(),
        kuaishou: kuaishou,
      );
      final description = model.content(data, 'description').trim();
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: Text(resourceTitle(model, item)),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                NativeVideoPoster(data: data, fallbackIDs: fallbackIDs),
                const SizedBox(height: 12),
                if (description.isNotEmpty) Text(description),
                Text(
                  failure ??
                      syncText(
                        context,
                        '此视频在快手播放。',
                        'This video plays on Kuaishou.',
                      ),
                ),
                if (uri == null) Text(model.t('invalid_video_link')),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text(model.t('close')),
            ),
            if (uri != null)
              FilledButton(
                onPressed: () async {
                  try {
                    if (await launchUrl(
                      uri,
                      mode: LaunchMode.externalApplication,
                    )) {
                      return;
                    }
                  } catch (_) {}
                  if (dialogContext.mounted) {
                    ScaffoldMessenger.of(dialogContext).showSnackBar(
                      SnackBar(
                        content: Text(
                          syncText(
                            dialogContext,
                            '暂时无法打开原视频。',
                            'Could not open the original video.',
                          ),
                        ),
                      ),
                    );
                  }
                },
                child: Text(
                  kuaishou
                      ? syncText(context, '前往快手观看', 'Watch on Kuaishou')
                      : syncText(context, '前往抖音观看', 'Watch on Douyin'),
                ),
              ),
          ],
        ),
      );
    } else {
      await NativeVideoSessions.stopAll();
      if (!context.mounted || !active()) return;
      await Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => NativeResourceViewer(
            item: item,
            private: private,
            autoStart: item['kind'] == 'video',
            onClosed: releaseOpening,
          ),
        ),
      );
    }
  } catch (e) {
    if (context.mounted && active()) showError(context, e);
  } finally {
    releaseOpening();
  }
}

class NativeVideoAddScreen extends StatelessWidget {
  final int? productID;
  const NativeVideoAddScreen({super.key, this.productID});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    Future<void> select(String source) async {
      final result = await Navigator.push(
        context,
        MaterialPageRoute(
          builder: (_) => source != 'LOCAL'
              ? NativeDouyinEditor(productID: productID, source: source)
              : NativeResourceEditor(kind: 'video', productID: productID),
        ),
      );
      if (context.mounted && result == true) Navigator.pop(context, true);
    }

    return Scaffold(
      appBar: appBar(
        context,
        productID != null
            ? syncText(context, '添加产品视频', 'Add product video')
            : syncText(context, '添加企业宣传视频', 'Add company video'),
      ),
      body: Padding(
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const Text('请选择添加方式'),
            space(),
            FutureBuilder<Json>(
              future: m.clientCapabilities(),
              builder: (context, snapshot) => Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  OutlinedButton(
                    onPressed: snapshot.data?['douyin_video'] == true
                        ? () => select('DOUYIN')
                        : null,
                    child: Text(
                      (snapshot.data?['video_api_version'] as num? ?? 2) >= 3
                          ? '抖音分享链接'
                          : '粘贴视频链接',
                    ),
                  ),
                  if ((snapshot.data?['video_api_version'] as num? ?? 2) >= 3)
                    OutlinedButton(
                      onPressed: () => select('KUAISHOU'),
                      child: const Text('快手分享链接'),
                    ),
                  OutlinedButton(
                    onPressed: () => select('LOCAL'),
                    child: const Text('上传本地视频'),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class NativeDouyinEditor extends StatefulWidget {
  final Json? item;
  final int? productID;
  final String source;
  const NativeDouyinEditor({
    super.key,
    this.item,
    this.productID,
    this.source = 'DOUYIN',
  });
  @override
  State<NativeDouyinEditor> createState() => _NativeDouyinEditorState();
}

class _NativeDouyinEditorState extends State<NativeDouyinEditor> {
  final title = TextEditingController(),
      share = TextEditingController(),
      description = TextEditingController();
  Json? current, check;
  Json data = {};
  int generation = 0, ownerID = 0;
  String endpoint = '', error = '', notice = '', errorCode = '';
  bool initialized = false, busy = false, checking = false;
  bool checkLock = false, saveLock = false;
  String lastShareText = '';
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (initialized) return;
    initialized = true;
    final m = ModelScope.of(context);
    ownerID = m.account['id'] ?? 0;
    endpoint = m.baseUrl;
    current = widget.item;
    data = widget.item == null
        ? {
            'title': '',
            'title_en': '',
            'description': '',
            'description_en': '',
            'format': 'video',
            'purpose': widget.productID == null ? 'company' : 'product',
            'product_id': widget.productID ?? 0,
            'file_media_id': 0,
            'cover_media_id': 0,
            'page_media_ids': <int>[],
            'video_source': widget.source,
            'original_share_url': '',
          }
        : asMap(widget.item!['data']);
    if (widget.productID != null) {
      data['product_id'] = widget.productID;
      data['purpose'] = 'product';
    }
    title.text = (data['title'] ?? '').toString();
    share.text = (data['original_share_url'] ?? '').toString();
    lastShareText = share.text;
    description.text = (data['description'] ?? '').toString();
    share.addListener(() {
      if (share.text == lastShareText) return;
      lastShareText = share.text;
      setState(() {
        generation++;
        check = null;
        error = '';
      });
    });
  }

  Future<bool> sessionReady() => nativeSessionReady(context, ownerID, endpoint);
  Future<void> inspect() async {
    if (checkLock) return;
    checkLock = true;
    if (!await sessionReady() || !mounted) {
      checkLock = false;
      return;
    }
    final g = ++generation;
    setState(() {
      checking = true;
      error = '';
      notice = '';
    });
    try {
      final r = asMap(
        await ModelScope.of(context).api(
          await ModelScope.of(context).videoPath('/merchant/video-links/check'),
          method: 'POST',
          data: {'text': share.text, 'source': data['video_source']},
        ),
      );
      if (!mounted || g != generation) return;
      setState(() {
        check = asMap(r['check']);
        if (title.text.trim().isEmpty) {
          title.text = (check!['title'] ?? '').toString();
        }
      });
    } catch (e) {
      if (mounted && g == generation) setState(() => error = videoErrorText(e));
    } finally {
      checkLock = false;
      if (mounted) setState(() => checking = false);
    }
  }

  Future<void> uploadCover() async {
    if (busy || !await sessionReady() || !mounted) return;
    final m = ModelScope.of(context);
    setState(() {
      busy = true;
      error = '';
    });
    try {
      final files = await FilePicker.pickFiles(
        type: FileType.custom,
        allowedExtensions: ['jpg', 'jpeg', 'png', 'webp'],
      );
      if (files.isEmpty || !mounted || !await sessionReady() || !mounted) {
        return;
      }
      final file = files.first;
      final path = file.path;
      if (path == null) throw ApiFailure('invalid_image');
      if ((await file.length() ?? 0) > 10 * 1024 * 1024) {
        throw ApiFailure('file_too_large');
      }
      final id = await m.upload(path, purpose: 'resource_cover');
      if (mounted && m.baseUrl == endpoint && m.account['id'] == ownerID) {
        setState(() => data['cover_media_id'] = id);
      }
    } catch (e) {
      if (mounted) {
        setState(() => error = m.t(e is ApiFailure ? e.code : 'invalid_image'));
      }
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> save(bool submit) async {
    if (saveLock) return;
    saveLock = true;
    if (!await sessionReady() || !mounted) {
      saveLock = false;
      return;
    }
    setState(() {
      busy = true;
      error = '';
      notice = '';
      errorCode = '';
    });
    try {
      if (widget.productID != null) {
        data['product_id'] = widget.productID;
        data['purpose'] = 'product';
      }
      if (submit && check == null) throw ApiFailure('video_check_required');
      if (submit && !canPreviewVideo(check)) {
        throw ApiFailure(
          (check!['reason'] ?? 'video_check_required').toString(),
        );
      }
      if (title.text.trim().isEmpty) {
        setState(() => error = '请填写视频名称。');
        return;
      }
      final payload = {
        ...data,
        'title': title.text.trim(),
        'description': description.text,
        'original_share_url': share.text,
      }..remove('rights_confirmed');
      final m = ModelScope.of(context);
      final result = asMap(
        await m.api(
          await m.videoPath(
            current == null
                ? '/merchant/resources'
                : '/merchant/resources/${current!['id']}/draft',
          ),
          method: current == null ? 'POST' : 'PUT',
          data: current == null
              ? {'kind': 'video', 'data': payload}
              : {'revision_id': current!['revision_id'], 'data': payload},
        ),
      );
      if (!mounted) return;
      current = result;
      data = asMap(result['data']);
      if (submit) {
        if (!await sessionReady() || !mounted) return;
        await m.api(
          await m.videoPath('/merchant/resources/${result['id']}/submit'),
          method: 'POST',
          data: {'revision_id': result['revision_id']},
        );
        if (mounted) Navigator.pop(context, true);
      } else {
        setState(() => notice = '草稿已保存，提交审核通过后才会公开。');
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          error = videoErrorText(e);
          errorCode = e is ApiFailure ? e.code : '';
        });
      }
    } finally {
      saveLock = false;
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  void dispose() {
    generation++;
    title.dispose();
    share.dispose();
    description.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final disabled = busy || current?['status'] == 'pending';
    final kuaishou = data['video_source'] == 'KUAISHOU';
    final platform = kuaishou ? '快手' : '抖音';
    final cover = (data['cover_media_id'] as num? ?? 0).toInt();
    return Scaffold(
      appBar: appBar(context, '粘贴视频链接'),
      body: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          Text(
            '视频来源：$platform',
            style: const TextStyle(fontWeight: FontWeight.bold),
          ),
          space(),
          TextField(
            controller: share,
            enabled: !disabled,
            maxLength: 8000,
            maxLines: 3,
            decoration: InputDecoration(
              labelText: '视频分享链接',
              hintText: '粘贴从$platform复制的分享链接或分享内容',
            ),
          ),
          if (kuaishou) const Text('链接检查仅验证格式。审核时会确认作品可公开打开。'),
          Wrap(
            spacing: 12,
            runSpacing: 8,
            children: [
              OutlinedButton(
                onPressed: disabled || checking ? null : inspect,
                child: Text(checking ? '正在检查…' : '检查链接'),
              ),
              OutlinedButton(
                onPressed: disabled || !canPreviewVideo(check)
                    ? null
                    : () => openNativeResource(
                        context,
                        {
                          'id': current?['id'] ?? 0,
                          'kind': 'video',
                          'data': {
                            ...data,
                            'title': title.text.trim(),
                            'description': description.text,
                          },
                        },
                        private: true,
                        check: check,
                      ),
                child: const Text('预览播放'),
              ),
            ],
          ),
          if (check != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 12),
              child: Text(
                check!['status'] == 'EXTERNAL_ONLY'
                    ? '链接格式正确，访客可前往快手观看。'
                    : check!['status'] == 'READY'
                    ? '官方播放器可预览，请点击预览确认播放效果。'
                    : '${videoErrorText(ApiFailure((check!['reason'] ?? '').toString()))} 可以先保存草稿，暂不能提交审核。',
              ),
            ),
          space(),
          TextField(
            controller: title,
            enabled: !disabled,
            maxLength: 200,
            decoration: const InputDecoration(labelText: '视频名称'),
          ),
          TextField(
            controller: description,
            enabled: !disabled,
            maxLength: 10000,
            maxLines: 3,
            decoration: InputDecoration(labelText: m.t('videoDescription')),
          ),
          if (widget.productID != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Text(
                syncText(
                  context,
                  '关联当前产品：#${widget.productID}（不可更改）',
                  'Linked to this product: #${widget.productID} (fixed)',
                ),
              ),
            )
          else
            ExpansionTile(
              title: Text(
                syncText(context, '关联产品（选填）', 'Linked product (optional)'),
              ),
              subtitle: Text(
                (data['product_id'] ?? 0) == 0
                    ? syncText(context, '不关联产品', 'No linked product')
                    : '#${data['product_id']}',
              ),
              children: [
                NativeProductPicker(
                  selected: (data['product_id'] as num? ?? 0).toInt(),
                  enabled: !disabled,
                  onSelected: (id) => setState(() {
                    data['product_id'] = id;
                    data['purpose'] = id > 0 ? 'product' : 'company';
                  }),
                ),
              ],
            ),
          if (cover > 0) Photo(id: cover, aspectRatio: 16 / 9),
          Wrap(
            spacing: 12,
            children: [
              OutlinedButton(
                onPressed: disabled ? null : uploadCover,
                child: Text(m.t('videoCover')),
              ),
              if (cover > 0)
                TextButton(
                  onPressed: disabled
                      ? null
                      : () => setState(() => data['cover_media_id'] = 0),
                  child: Text(m.t('remove')),
                ),
            ],
          ),
          NativeVideoPurpose(
            value: (data['purpose'] ?? 'company').toString(),
            onChanged: disabled || widget.productID != null
                ? null
                : (value) => setState(() => data['purpose'] = value),
          ),
          const NativePublicationAgreements(hideAccepted: true),
          const Text('视频保存在原平台，上传的封面计入展厅存储。修改审核期间继续展示已公开的版本。'),
          space(),
          if (error.isNotEmpty) message(context, error),
          if (errorCode == 'duplicate_video' && widget.productID != null) ...[
            Text(
              syncText(
                context,
                '一条视频最多关联一个产品。请在企业视频管理中调整已有视频的关联产品，再提交审核。',
                'A video can be linked to one product. Edit the existing video in company video management and resubmit.',
              ),
            ),
            TextButton(
              onPressed: () =>
                  push(context, const MerchantResourcesScreen(kind: 'video')),
              child: Text(
                syncText(context, '去企业视频管理', 'Manage company videos'),
              ),
            ),
          ],
          if (notice.isNotEmpty) message(context, notice),
          Wrap(
            spacing: 12,
            runSpacing: 8,
            children: [
              OutlinedButton(
                onPressed: disabled || checking ? null : () => save(false),
                child: const Text('保存云端草稿'),
              ),
              FilledButton(
                onPressed: disabled || checking || !canPreviewVideo(check)
                    ? null
                    : () => save(true),
                child: const Text('提交审核'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}
