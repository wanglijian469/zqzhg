part of 'main.dart';

String videoV2(String path) =>
    '$path${path.contains('?') ? '&' : '?'}video_api_version=2';
bool isDouyinVideo(Json item) =>
    item['kind'] == 'video' && item['data']?['video_source'] == 'DOUYIN';
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
          u.queryParameters['autoplay'] == '0'
      ? u
      : null;
}

String videoErrorText(Object e) {
  final code = e is ApiFailure ? e.code : e.toString();
  return const <String, String>{
        'video_check_required': '请先检查链接并确认能够预览，再提交审核。',
        'video_rights_required': '请确认您有权展示该视频。',
        'invalid_video_link': '请输入有效的抖音视频分享链接。',
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

Future<void> openOfficialVideo(Json check) async {
  final id = (check['video_id'] ?? '').toString();
  final url = trustedDouyinPlayer((check['player_url'] ?? '').toString(), id);
  if (check['status'] != 'READY' || url == null) {
    throw ApiFailure((check['reason'] ?? 'video_check_required').toString());
  }
  // Only the validated public player URL and dimensions cross into WebView. No login token.
  await const MethodChannel('com.zqzt.dalunongji/video').invokeMethod('open', {
    'url': url.toString(),
    'width': check['width'] ?? 0,
    'height': check['height'] ?? 0,
  });
}

class NativeVideoAddScreen extends StatelessWidget {
  final int? productID;
  const NativeVideoAddScreen({super.key, this.productID});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    Future<void> select(bool link) async {
      final result = await Navigator.push(
        context,
        MaterialPageRoute(
          builder: (_) => link
              ? NativeDouyinEditor(productID: productID)
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
                        ? () => select(true)
                        : null,
                    child: const Text('粘贴视频链接'),
                  ),
                  OutlinedButton(
                    onPressed: () => select(false),
                    child: const Text('上传本地视频'),
                  ),
                ],
              ),
            ),
            space(),
            const Text('粘贴链接仅支持抖音公开视频；其他视频请使用原本地上传功能。'),
          ],
        ),
      ),
    );
  }
}

class NativeDouyinEditor extends StatefulWidget {
  final Json? item;
  final int? productID;
  const NativeDouyinEditor({super.key, this.item, this.productID});
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
            'video_source': 'DOUYIN',
            'original_share_url': '',
            'rights_confirmed': false,
          }
        : asMap(widget.item!['data']);
    if (widget.productID != null) {
      data['product_id'] = widget.productID;
      data['purpose'] = 'product';
    }
    title.text = (data['title'] ?? '').toString();
    share.text = (data['original_share_url'] ?? '').toString();
    description.text = (data['description'] ?? '').toString();
    share.addListener(
      () => setState(() {
        generation++;
        checking = false;
        check = null;
        data['rights_confirmed'] = false;
        error = '';
      }),
    );
  }

  Future<bool> sessionReady() => nativeSessionReady(context, ownerID, endpoint);
  Future<void> inspect() async {
    if (!await sessionReady() || !mounted) return;
    final g = ++generation;
    setState(() {
      checking = true;
      error = '';
      notice = '';
    });
    try {
      final r = asMap(
        await ModelScope.of(context).api(
          videoV2('/merchant/video-links/check'),
          method: 'POST',
          data: {'text': share.text},
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
      if (mounted && g == generation) setState(() => checking = false);
    }
  }

  Future<void> save(bool submit) async {
    if (!await sessionReady() || !mounted) return;
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
      if (check == null) throw ApiFailure('video_check_required');
      if (data['rights_confirmed'] != true) {
        throw ApiFailure('video_rights_required');
      }
      if (submit && check!['status'] != 'READY') {
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
      };
      final m = ModelScope.of(context);
      final result = asMap(
        await m.api(
          videoV2(
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
          videoV2('/merchant/resources/${result['id']}/submit'),
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
    final disabled = busy || current?['status'] == 'pending';
    return Scaffold(
      appBar: appBar(context, '粘贴视频链接'),
      body: ListView(
        padding: const EdgeInsets.all(20),
        children: [
          const Text('视频来源：抖音', style: TextStyle(fontWeight: FontWeight.bold)),
          space(),
          TextField(
            controller: share,
            enabled: !disabled,
            maxLength: 8000,
            maxLines: 3,
            decoration: const InputDecoration(
              labelText: '视频分享链接',
              hintText: '粘贴从抖音复制的分享链接或分享内容',
            ),
          ),
          Wrap(
            spacing: 12,
            runSpacing: 8,
            children: [
              OutlinedButton(
                onPressed: disabled || checking ? null : inspect,
                child: Text(checking ? '正在检查…' : '检查链接'),
              ),
              OutlinedButton(
                onPressed: disabled || check?['status'] != 'READY'
                    ? null
                    : () async {
                        try {
                          await openOfficialVideo(check!);
                        } catch (e) {
                          if (mounted) {
                            setState(() => error = videoErrorText(e));
                          }
                        }
                      },
                child: const Text('预览播放'),
              ),
            ],
          ),
          if (check != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 12),
              child: Text(
                check!['status'] == 'READY'
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
            decoration: const InputDecoration(labelText: '视频说明'),
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
          CheckboxListTile(
            contentPadding: EdgeInsets.zero,
            value: data['rights_confirmed'] == true,
            onChanged: disabled
                ? null
                : (v) => setState(() => data['rights_confirmed'] = v == true),
            title: const Text('我确认该视频由本人发布，或已取得合法展示授权。'),
          ),
          const Text('视频仍保存在抖音，不占展厅的视频文件空间。资料修改后需重新审核，已公开的旧版本继续展示。'),
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
                onPressed: disabled || checking || check?['status'] != 'READY'
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
