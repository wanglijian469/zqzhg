part of 'main.dart';

String serviceState(String value, bool zh) =>
    <String, List<String>>{
      'draft': ['草稿', 'Draft'],
      'pending': ['待审核', 'Pending review'],
      'approved': ['已通过', 'Approved'],
      'rejected': ['已驳回', 'Rejected'],
      'revoked': ['已撤销', 'Revoked'],
      'received': ['已受理', 'Received'],
      'processing': ['处理中', 'Processing'],
      'needs_info': ['待补充', 'More information needed'],
      'closed': ['已结案', 'Closed'],
    }[value]?[zh ? 0 : 1] ??
    value;
String newServiceCredential() {
  final r = Random.secure();
  return List.generate(
    32,
    (_) => r.nextInt(256).toRadixString(16).padLeft(2, '0'),
  ).join();
}

List<Json> serviceAgreementRefs(List<Json> docs) => docs
    .map(
      (d) => <String, dynamic>{
        'key': d['key'],
        'version': d['version'],
        'digest': d['digest'],
      },
    )
    .toList();

class NativeServiceLinks extends StatelessWidget {
  const NativeServiceLinks({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    return Column(
      children: [
        ListTile(
          leading: const Icon(Icons.info_outline),
          title: Text(zh ? '入驻与服务说明' : 'Admission and services'),
          onTap: () => push(context, const NativeServicesScreen()),
        ),
        if (isMerchant(m))
          ListTile(
            leading: const Icon(Icons.verified_user_outlined),
            title: Text(zh ? '协议与提交' : 'Agreements and submission'),
            onTap: () => push(context, const NativeVerificationScreen()),
          ),
        ListTile(
          leading: const Icon(Icons.support_agent),
          title: Text(zh ? '投诉与服务申请' : 'Complaints and service requests'),
          onTap: () => push(context, const NativeServiceRequestScreen()),
        ),
        ListTile(
          leading: const Icon(Icons.description_outlined),
          title: Text(zh ? '用户协议' : 'User agreement'),
          onTap: () =>
              push(context, const NativeServicesScreen(documentKey: 'terms')),
        ),
        ListTile(
          leading: const Icon(Icons.privacy_tip_outlined),
          title: Text(zh ? '隐私政策' : 'Privacy policy'),
          onTap: () =>
              push(context, const NativeServicesScreen(documentKey: 'privacy')),
        ),
        SwitchListTile(
          value: m.prefs.getBool('service.analytics_consent') == true,
          title: Text(zh ? '可选访问统计' : 'Optional usage analytics'),
          subtitle: Text(
            zh
                ? '默认关闭，可随时撤回，不影响基本浏览。'
                : 'Off by default. Withdrawal does not affect basic browsing.',
          ),
          onChanged: (value) async {
            await m.prefs.setBool('service.analytics_consent', value);
            if (!value) await m.analytics.withdraw();
            m.favoritesChanged();
          },
        ),
      ],
    );
  }
}

class NativeServicesScreen extends StatefulWidget {
  final String documentKey;
  const NativeServicesScreen({super.key, this.documentKey = 'service-info'});
  @override
  State<NativeServicesScreen> createState() => _NativeServicesScreenState();
}

class _NativeServicesScreenState extends State<NativeServicesScreen> {
  int? version;
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    return Scaffold(
      appBar: appBar(context, zh ? '服务说明与协议' : 'Services and agreements'),
      body: SafeArea(
        child: AsyncPane(
          identity: '${m.baseUrl}:${widget.documentKey}',
          load: () async {
            final active = asMap(await m.api('/public/service-documents'));
            final history = asMap(
              await m.api(
                '/public/service-documents/${widget.documentKey}/history',
              ),
            );
            return <String, dynamic>{
              'active': rows(
                active['items'],
              ).where((d) => d['key'] == widget.documentKey).firstOrNull,
              'history': rows(history['items']),
            };
          },
          builder: (result) {
            final data = asMap(result), history = rows(data['history']);
            final document = version == null
                ? asMap(data['active'] ?? {})
                : history.where((d) => d['version'] == version).firstOrNull ??
                      <String, dynamic>{};
            return ListView(
              padding: const EdgeInsets.all(20),
              children: [
                if (document.isNotEmpty)
                  NativeServiceDocument(document: document),
                DropdownButtonFormField<int>(
                  initialValue: version,
                  decoration: InputDecoration(
                    labelText: zh ? '查看历史版本' : 'Historical versions',
                  ),
                  items: [
                    DropdownMenuItem(
                      value: null,
                      child: Text(zh ? '当前生效版本' : 'Current effective version'),
                    ),
                    ...history.map(
                      (d) => DropdownMenuItem(
                        value: (d['version'] as num).toInt(),
                        child: Text('v${d['version']}'),
                      ),
                    ),
                  ],
                  onChanged: (v) => setState(() => version = v),
                ),
                space(),
                if (widget.documentKey != 'showroom-agreement')
                  TextButton(
                    onPressed: () => push(
                      context,
                      const NativeServicesScreen(
                        documentKey: 'showroom-agreement',
                      ),
                    ),
                    child: Text(zh ? '企业展厅服务协议' : 'Showroom service agreement'),
                  ),
                const NativeServiceLinks(),
              ],
            );
          },
        ),
      ),
    );
  }
}

class NativeServiceDocument extends StatelessWidget {
  final Json document;
  const NativeServiceDocument({super.key, required this.document});
  @override
  Widget build(BuildContext context) {
    final zh = ModelScope.of(context).language == 'zh';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          (document[zh ? 'title' : 'title_en'] ?? '').toString(),
          style: Theme.of(context).textTheme.headlineSmall,
        ),
        space(8),
        Text(
          '${zh ? '版本' : 'Version'} ${document['version']} · ${(document['effective_at'] ?? '').toString().split('T').first}',
        ),
        space(),
        SelectableText(
          (document[zh ? 'body' : 'body_en'] ?? '').toString(),
          style: const TextStyle(height: 1.8),
        ),
        space(24),
      ],
    );
  }
}

class NativeRegistrationAgreements extends StatefulWidget {
  final ValueChanged<List<Json>> onChanged;
  final bool initiallyAccepted;
  const NativeRegistrationAgreements({
    super.key,
    required this.onChanged,
    this.initiallyAccepted = false,
  });
  @override
  State<NativeRegistrationAgreements> createState() =>
      _NativeRegistrationAgreementsState();
}

class _NativeRegistrationAgreementsState
    extends State<NativeRegistrationAgreements> {
  bool agreed = false;
  @override
  void initState() {
    super.initState();
    agreed = widget.initiallyAccepted;
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    return AsyncPane(
      identity: 'register-docs:${m.baseUrl}',
      load: () => m.api('/public/service-documents'),
      builder: (result) {
        final docs = rows(
          asMap(result)['items'],
        ).where((d) => ['terms', 'privacy'].contains(d['key'])).toList();
        return Column(
          children: [
            CheckboxListTile(
              contentPadding: EdgeInsets.zero,
              value: agreed,
              title: Text(
                zh
                    ? '我已阅读并同意用户协议及隐私政策'
                    : 'I have read and accept the user agreement and privacy policy',
              ),
              onChanged: (v) {
                setState(() => agreed = v == true);
                widget.onChanged(agreed ? serviceAgreementRefs(docs) : []);
              },
            ),
            Wrap(
              children: [
                TextButton(
                  onPressed: () => push(
                    context,
                    const NativeServicesScreen(documentKey: 'terms'),
                  ),
                  child: Text(zh ? '用户协议' : 'User agreement'),
                ),
                TextButton(
                  onPressed: () => push(
                    context,
                    const NativeServicesScreen(documentKey: 'privacy'),
                  ),
                  child: Text(zh ? '隐私政策' : 'Privacy policy'),
                ),
              ],
            ),
            Text(
              zh
                  ? '基础入驻永久免费，默认免费存储200 MB。'
                  : 'Permanently free basic admission; default free storage 200 MB.',
            ),
          ],
        );
      },
    );
  }
}

Future<int> uploadServiceEvidence(
  AppModel m,
  PlatformFile file,
  String purpose,
  String credential,
) async {
  final request = http.MultipartRequest(
    'POST',
    Uri.parse('${m.baseUrl}/api/v1/public/service-evidence'),
  );
  if (m.token.isNotEmpty) {
    request.headers['Authorization'] = 'Bearer ${m.token}';
  }
  if (credential.isNotEmpty) {
    request.headers['X-Service-Credential'] = credential;
  }
  request.fields['purpose'] = purpose;
  final bytes = await file.readAsBytes();
  if (bytes.length > 10000000) throw ApiFailure('invalid_evidence');
  request.files.add(
    http.MultipartFile.fromBytes('file', bytes, filename: file.name),
  );
  final response = await http.Response.fromStream(
    await m.client.send(request).timeout(const Duration(seconds: 60)),
  ).timeout(const Duration(seconds: 60));
  final result = asMap(jsonDecode(response.body));
  if (response.statusCode >= 400) {
    throw ApiFailure(asMap(result['error'])['code'].toString());
  }
  return (result['id'] as num).toInt();
}

Future<List<PlatformFile>> pickServiceEvidence({bool multiple = false}) async {
  final List<PlatformFile> files;
  if (multiple) {
    files = await FilePicker.pickFiles(
      type: FileType.custom,
      allowedExtensions: ['jpg', 'jpeg', 'png', 'webp', 'pdf'],
    );
  } else {
    final selected = await FilePicker.pickFile(
      type: FileType.custom,
      allowedExtensions: ['jpg', 'jpeg', 'png', 'webp', 'pdf'],
    );
    files = selected == null ? [] : [selected];
  }
  if (files.length > 5) throw ApiFailure('invalid_evidence');
  for (final file in files) {
    final size = await file.length();
    if (size == null || size > 10000000) throw ApiFailure('invalid_evidence');
  }
  return files;
}

Future<void> downloadServiceEvidence(
  BuildContext context,
  int id, {
  String credential = '',
}) async {
  final m = ModelScope.of(context);
  try {
    final response = await m.client
        .get(
          Uri.parse('${m.baseUrl}/api/v1/public/service-evidence/$id'),
          headers: {
            if (m.token.isNotEmpty) 'Authorization': 'Bearer ${m.token}',
            if (credential.isNotEmpty) 'X-Service-Credential': credential,
          },
        )
        .timeout(const Duration(seconds: 30));
    if (response.statusCode != 200) throw ApiFailure('not_found');
    final mime = response.headers['content-type'] ?? '';
    final extension = mime.contains('pdf')
        ? 'pdf'
        : mime.contains('png')
        ? 'png'
        : mime.contains('webp')
        ? 'webp'
        : 'jpg';
    await FilePicker.saveFile(
      dialogTitle: m.t('download'),
      fileName: 'evidence-$id.$extension',
      bytes: response.bodyBytes,
    );
  } catch (e) {
    if (context.mounted) showError(context, e);
  }
}

class NativeFreeStorage extends StatelessWidget {
  const NativeFreeStorage({super.key});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    String mb(num n) => '${(n / 1000000).toStringAsFixed(2)} MB';
    return AsyncPane(
      identity: 'storage:${m.merchant.key}',
      load: () => m.api('/merchant/storage/usage'),
      builder: (result) {
        final r = asMap(result),
            used = (r['used_bytes'] as num?) ?? 0,
            reserved = (r['reserved_bytes'] as num?) ?? 0,
            quota = (r['quota_bytes'] as num?) ?? 200000000,
            remaining = max(
              0,
              (r['remaining_bytes'] as num?) ?? quota - used - reserved,
            );
        return Card(
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  zh ? '免费存储用量' : 'Free storage usage',
                  style: Theme.of(context).textTheme.titleMedium,
                ),
                space(8),
                Text(
                  '${zh ? '已用' : 'Used'} ${mb(used)} / ${zh ? '额度' : 'Allowance'} ${mb(quota)}',
                ),
                Text(
                  '${zh ? '上传预占' : 'Reserved'} ${mb(reserved)} · ${zh ? '剩余' : 'Available'} ${mb(remaining)}',
                ),
                space(8),
                LinearProgressIndicator(
                  value: quota > 0 ? min(1, (used + reserved) / quota) : 1,
                ),
                space(8),
                Text(
                  zh
                      ? '默认免费额度200 MB，企业调整以本页为准。'
                      : 'Default free allowance: 200 MB. Your allowance is shown above.',
                ),
                if ((used + reserved) * 10 >= quota * 9)
                  message(
                    context,
                    zh
                        ? '免费存储接近或已达到额度；优化资料或联系管理员。现有合法资料保留，不自动扣费、不提供付费扩容。'
                        : 'Storage is nearing or exceeding allowance. Optimize materials or contact an administrator. Existing lawful files remain without charges or paid expansion.',
                  ),
                TextButton(
                  onPressed: () => push(
                    context,
                    const NativeServiceRequestScreen(initialKind: 'storage'),
                  ),
                  child: Text(
                    zh
                        ? '联系管理员／免费资源调整'
                        : 'Contact administrator / free adjustment',
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}

/// Legacy entry opens the existing enterprise editor at the agreement step.
class NativeVerificationScreen extends StatelessWidget {
  const NativeVerificationScreen({super.key});
  @override
  Widget build(BuildContext context) => MerchantVendorPane(
    builder: (item) =>
        EditorScreen(kind: 'vendor', item: item, initialSection: 'review'),
  );
}

class NativePublicationAgreements extends StatefulWidget {
  const NativePublicationAgreements({super.key});
  @override
  State<NativePublicationAgreements> createState() =>
      _NativePublicationAgreementsState();
}

class _NativePublicationAgreementsState
    extends State<NativePublicationAgreements> {
  List<Json> documents = [], acceptances = [];
  String? source;
  String error = '';
  bool loading = true, busy = false, agreed = false;
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    final next = '${m.scope}:${m.token}';
    if (source != next) {
      source = next;
      loading = true;
      agreed = false;
      unawaited(load());
    }
  }

  Future<void> load() async {
    final m = ModelScope.of(context), expected = source;
    try {
      final result = asMap(await m.api('/merchant/verification'));
      if (!mounted || source != expected) return;
      setState(() {
        documents = rows(
          result['documents'],
        ).where((d) => d['key'] != 'service-info').toList();
        acceptances = rows(result['acceptances']);
        loading = false;
        error = '';
        agreed = false;
      });
    } catch (e) {
      if (mounted && source == expected) {
        setState(() {
          error = e is ApiFailure ? e.code : 'network_error';
          loading = false;
        });
      }
    }
  }

  Future<void> accept() async {
    if (busy || !agreed || documents.length != 3) return;
    final m = ModelScope.of(context), expected = source;
    setState(() => busy = true);
    try {
      await m.api(
        '/merchant/service-acceptances',
        method: 'POST',
        data: {
          'agreements': serviceAgreementRefs(documents),
          'source': Platform.isIOS ? 'ios' : 'android',
        },
      );
      if (!mounted || source != expected) return;
      await load();
    } catch (e) {
      if (mounted && source == expected) {
        setState(() => error = e is ApiFailure ? e.code : 'network_error');
      }
    } finally {
      if (mounted && source == expected) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    final accepted =
        documents.length == 3 &&
        documents.every(
          (d) => acceptances.any(
            (a) => a['document_id'] == d['id'] && a['digest'] == d['digest'],
          ),
        );
    if (m.token.isEmpty) {
      return Text(
        zh ? '注册／登录后可确认协议并提交资料。' : 'Sign in to accept agreements and submit.',
      );
    }
    if (loading) {
      return const Padding(
        padding: EdgeInsets.all(16),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(
              zh ? '服务协议确认' : 'Service agreements',
              style: Theme.of(context).textTheme.titleMedium,
            ),
            if (error.isNotEmpty) ...[
              Text(m.t(error)),
              TextButton(
                onPressed: busy ? null : load,
                child: Text(m.t('retry')),
              ),
            ],
            for (final d in documents)
              ExpansionTile(
                title: Text(
                  '${d[zh ? 'title' : 'title_en']} · v${d['version']}',
                ),
                children: [
                  Padding(
                    padding: const EdgeInsets.all(12),
                    child: NativeServiceDocument(document: d),
                  ),
                ],
              ),
            if (accepted)
              Text(zh ? '已确认当前版本协议' : 'Current agreements accepted')
            else ...[
              CheckboxListTile(
                contentPadding: EdgeInsets.zero,
                value: agreed,
                onChanged: busy
                    ? null
                    : (v) => setState(() => agreed = v == true),
                title: Text(
                  zh
                      ? '我已阅读并同意以上当前版本协议'
                      : 'I have read and accept the current agreements above',
                ),
              ),
              FilledButton(
                onPressed: busy || !agreed || documents.length != 3
                    ? null
                    : accept,
                child: Text(zh ? '确认协议' : 'Accept agreements'),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class NativeServiceRequestScreen extends StatefulWidget {
  final String initialKind;
  const NativeServiceRequestScreen({super.key, this.initialKind = 'complaint'});
  @override
  State<NativeServiceRequestScreen> createState() =>
      _NativeServiceRequestScreenState();
}

class _NativeServiceRequestScreenState
    extends State<NativeServiceRequestScreen> {
  final controls = {
    for (final key in [
      'subject',
      'description',
      'target_url',
      'contact',
      'lookup_id',
      'lookup_credential',
      'supplement',
    ])
      key: TextEditingController(),
  };
  late String kind = widget.initialKind;
  String credential = newServiceCredential(),
      requestID = newServiceCredential(),
      error = '';
  List<PlatformFile> files = [];
  List<int> uploaded = [];
  Json? result;
  bool busy = false;
  int refresh = 0;
  @override
  void dispose() {
    for (final c in controls.values) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> run(Future<void> Function() task) async {
    if (busy) return;
    setState(() => busy = true);
    setState(() => error = '');
    try {
      await task();
    } catch (e) {
      if (mounted) {
        final m = ModelScope.of(context);
        setState(
          () => error = e is ApiFailure ? m.t(e.code) : m.t('network_error'),
        );
      }
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  Future<void> lookup() async {
    final m = ModelScope.of(context);
    final value = asMap(
      await m.api(
        '/public/service-requests/lookup',
        method: 'POST',
        data: {
          'id': int.tryParse(controls['lookup_id']!.text),
          'credential': controls['lookup_credential']!.text.trim(),
        },
      ),
    );
    if (mounted) setState(() => result = value);
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context),
        zh = m.language == 'zh',
        signed = isMerchant(m);
    final kinds = <String, List<String>>{
      'complaint': ['投诉举报', 'Complaint'],
      if (signed) ...{
        'appeal': ['申诉复核', 'Appeal'],
        'export': ['资料导出', 'Export'],
        'unpublish': ['停止展示', 'Unpublish'],
        'withdraw': ['退出平台', 'Withdrawal'],
        'account_close': ['账号注销', 'Account closure'],
        'storage': ['免费资源调整', 'Free storage adjustment'],
      },
    };
    if (!kinds.containsKey(kind)) kind = 'complaint';
    return Scaffold(
      appBar: appBar(
        context,
        zh ? '投诉与服务申请' : 'Complaints and service requests',
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            Text(
              zh
                  ? '投诉可匿名提交。企业权益申请需登录并核验归属，不因提交申请自动删除资料。请保存工单编号及私密查询凭证。'
                  : 'Complaints may be anonymous. Company rights requests require login and ownership checks. Submission never automatically deletes materials. Keep the ID and private credential.',
            ),
            space(),
            if (error.isNotEmpty) message(context, error),
            if (!signed)
              TextButton(
                onPressed: () =>
                    push(context, const LoginScreen(register: false)),
                child: Text(zh ? '企业登录' : 'Company sign-in'),
              ),
            DropdownButtonFormField<String>(
              initialValue: kind,
              items: kinds.entries
                  .map(
                    (e) => DropdownMenuItem(
                      value: e.key,
                      child: Text(e.value[zh ? 0 : 1]),
                    ),
                  )
                  .toList(),
              onChanged: busy
                  ? null
                  : (v) => setState(() => kind = v ?? 'complaint'),
              decoration: InputDecoration(
                labelText: zh ? '申请类型' : 'Request type',
              ),
            ),
            space(),
            ...<String, List<String>>{
              'subject': ['主题', 'Subject'],
              'description': ['详细说明', 'Details'],
              'target_url': ['相关网址（选填）', 'Related URL (optional)'],
              'contact': ['联系方式（选填）', 'Contact (optional)'],
            }.entries.map(
              (entry) => Padding(
                padding: const EdgeInsets.only(bottom: 16),
                child: TextField(
                  controller: controls[entry.key],
                  enabled: !busy,
                  maxLines: entry.key == 'description' ? 5 : 1,
                  maxLength: entry.key == 'description'
                      ? 10000
                      : entry.key == 'target_url'
                      ? 1000
                      : 200,
                  decoration: InputDecoration(
                    labelText: entry.value[zh ? 0 : 1],
                  ),
                ),
              ),
            ),
            OutlinedButton(
              onPressed: busy
                  ? null
                  : () => run(() async {
                      final selected = await pickServiceEvidence(
                        multiple: true,
                      );
                      if (mounted) {
                        setState(() {
                          files = selected;
                          uploaded = [];
                        });
                      }
                    }),
              child: Text(
                zh
                    ? '选择私有证据（最多5个，每个10 MB）'
                    : 'Choose private evidence (5 files, 10 MB each)',
              ),
            ),
            ...files.map((f) => Text(f.name)),
            space(),
            FilledButton(
              onPressed: busy
                  ? null
                  : () => run(() async {
                      if (controls['subject']!.text.trim().isEmpty ||
                          controls['description']!.text.trim().isEmpty) {
                        throw ApiFailure('invalid_request');
                      }
                      for (var i = uploaded.length; i < files.length; i++) {
                        uploaded.add(
                          await uploadServiceEvidence(
                            m,
                            files[i],
                            'request',
                            credential,
                          ),
                        );
                      }
                      final row = asMap(
                        await m.api(
                          '/public/service-requests',
                          method: 'POST',
                          data: {
                            'kind': kind,
                            ...{
                              for (final key in [
                                'subject',
                                'description',
                                'target_url',
                                'contact',
                              ])
                                key: controls[key]!.text.trim(),
                            },
                            'attachments': uploaded,
                            'credential': credential,
                            'request_id': requestID,
                          },
                        ),
                      );
                      if (mounted) {
                        setState(() {
                          result = row;
                          controls['lookup_id']!.text = row['id'].toString();
                          controls['lookup_credential']!.text = credential;
                          refresh++;
                        });
                      }
                    }),
              child: Text(
                busy ? m.t('loading') : (zh ? '提交申请' : 'Submit request'),
              ),
            ),
            space(24),
            Text(
              zh ? '查询反馈' : 'Read replies',
              style: Theme.of(context).textTheme.titleLarge,
            ),
            TextField(
              controller: controls['lookup_id'],
              keyboardType: TextInputType.number,
              decoration: InputDecoration(
                labelText: zh ? '工单编号' : 'Request ID',
              ),
            ),
            space(),
            TextField(
              controller: controls['lookup_credential'],
              decoration: InputDecoration(
                labelText: zh ? '私密查询凭证' : 'Private lookup credential',
              ),
            ),
            TextButton(
              onPressed: busy ? null : () => run(lookup),
              child: Text(zh ? '查询' : 'Look up'),
            ),
            if (result != null) ...[
              SelectableText(
                '#${result!['id']} · ${result!['subject']}\n${serviceState(result!['status'].toString(), zh)}\n${result!['reply'] ?? ''}',
              ),
              space(),
              TextButton(
                onPressed: () => sharePublicText(
                  context,
                  jsonEncode({
                    'id': controls['lookup_id']!.text,
                    'credential': controls['lookup_credential']!.text,
                  }),
                  zh ? '保存私密查询凭证' : 'Keep private lookup credential',
                  jsonEncode({
                    'id': controls['lookup_id']!.text,
                    'credential': controls['lookup_credential']!.text,
                  }),
                ),
                child: Text(
                  zh ? '保存／分享查询凭证' : 'Save / share private credential',
                ),
              ),
              if (result!['status'] != 'closed') ...[
                TextField(
                  controller: controls['supplement'],
                  maxLines: 3,
                  maxLength: 2000,
                  decoration: InputDecoration(
                    labelText: zh ? '补充说明' : 'Additional information',
                  ),
                ),
                TextButton(
                  onPressed: busy
                      ? null
                      : () => run(() async {
                          await m.api(
                            '/public/service-requests/${result!['id']}/supplement',
                            method: 'POST',
                            data: {
                              'credential': controls['lookup_credential']!.text
                                  .trim(),
                              'description': controls['supplement']!.text,
                              'version': result!['version'],
                            },
                          );
                          controls['supplement']!.clear();
                          await lookup();
                        }),
                  child: Text(zh ? '提交补充' : 'Send information'),
                ),
              ],
              TextButton(
                onPressed: busy
                    ? null
                    : () => setState(() {
                        credential = newServiceCredential();
                        requestID = newServiceCredential();
                        files = [];
                        uploaded = [];
                        result = null;
                        for (final key in [
                          'subject',
                          'description',
                          'target_url',
                          'contact',
                        ]) {
                          controls[key]!.clear();
                        }
                      }),
                child: Text(zh ? '新建申请' : 'New request'),
              ),
            ],
            if (signed)
              AsyncPane(
                identity: 'requests:${m.scope}:$refresh',
                load: () => m.api('/merchant/service-requests'),
                builder: (value) => Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      zh ? '我的申请' : 'My requests',
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                    ...rows(asMap(value)['items']).map(
                      (r) => ListTile(
                        title: Text('#${r['id']} · ${r['subject']}'),
                        subtitle: Text(
                          '${serviceState(r['status'].toString(), zh)}\n${r['reply'] ?? ''}',
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            space(24),
          ],
        ),
      ),
    );
  }
}
