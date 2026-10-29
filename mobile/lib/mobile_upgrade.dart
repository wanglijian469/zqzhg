part of 'main.dart';

extension MobileCapabilities on AppModel {
  Future<Json> clientCapabilities({bool refresh = false}) {
    final source = baseUrl;
    if (refresh) clientCapabilityCache.remove(source);
    final cached = clientCapabilityCache[source];
    if (cached != null) return cached;
    final request = api('/public/config')
        .then<Json>((value) {
          if (baseUrl != source) throw ApiFailure('network_error');
          return asMap(value['capabilities'] ?? {});
        })
        .catchError((Object e) {
          clientCapabilityCache.remove(source);
          if (e is ApiFailure && e.status == 404) return <String, dynamic>{};
          throw e;
        });
    clientCapabilityCache[source] = request;
    return request;
  }
}

class NativeRegionField extends StatefulWidget {
  final Json data;
  final ValueChanged<Json> onChanged;
  final bool enabled;
  const NativeRegionField({
    super.key,
    required this.data,
    required this.onChanged,
    this.enabled = true,
  });
  @override
  State<NativeRegionField> createState() => _NativeRegionFieldState();
}

class _NativeRegionFieldState extends State<NativeRegionField> {
  Json? result;
  String? error, scope;
  bool loading = false;
  final manual = TextEditingController();
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    final key = '${m.baseUrl}:${widget.data['region_code'] ?? ''}';
    if (scope != key) {
      scope = key;
      unawaited(load());
    }
  }

  Future<void> load() async {
    final m = ModelScope.of(context), source = m.baseUrl, key = scope;
    setState(() => loading = true);
    try {
      final caps = await m.clientCapabilities();
      if (caps['administrative_regions'] != true) {
        if (mounted && scope == key) {
          setState(() {
            result = null;
            error = 'unsupported';
          });
        }
        return;
      }
      final code = widget.data['region_code']?.toString() ?? '';
      final r = asMap(
        await m.api(
          '/public/administrative-regions${code.isEmpty ? '' : '?code=${Uri.encodeQueryComponent(code)}'}',
        ),
      );
      if (mounted && m.baseUrl == source && scope == key) {
        setState(() {
          result = r;
          error = null;
        });
      }
    } catch (_) {
      if (mounted && scope == key) setState(() => error = 'network_error');
    } finally {
      if (mounted && scope == key) setState(() => loading = false);
    }
  }

  @override
  void dispose() {
    manual.dispose();
    super.dispose();
  }

  Future<void> choose(String parent, {Json? selected}) async {
    final m = ModelScope.of(context), source = m.baseUrl;
    Json r;
    try {
      r = asMap(
        await m.api(
          '/public/administrative-regions?parent_code=${Uri.encodeQueryComponent(parent)}',
        ),
      );
    } catch (e) {
      if (mounted) showError(context, e);
      return;
    }
    if (!mounted || m.baseUrl != source) return;
    final items = rows(r['items']);
    if (items.isEmpty) return;
    final node = await showModalBottomSheet<Json>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (c) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(c).height * .65,
          child: Column(
            children: [
              Text(
                syncText(c, '选择所在地区', 'Select a region'),
                style: const TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.bold,
                ),
              ),
              if (selected != null)
                ListTile(
                  title: Text(
                    syncText(
                      c,
                      '仅保留当前城市，不细分区县',
                      'Keep city without a district',
                    ),
                  ),
                  onTap: () => Navigator.pop(c, selected),
                ),
              Expanded(
                child: ListView.builder(
                  itemCount: items.length,
                  itemBuilder: (c, i) => ListTile(
                    title: Text(items[i]['name']),
                    trailing: items[i]['has_children'] == true
                        ? const Icon(Icons.chevron_right)
                        : null,
                    onTap: () => Navigator.pop(c, items[i]),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
    if (node == null || !mounted || m.baseUrl != source) return;
    if ((node['level'] as num) >= 2 || node['has_children'] != true) {
      widget.onChanged({
        'region_code': node['code'],
        'region_name': node['full_name'] ?? node['name'],
        'region_id': 0,
      });
    }
    if (node['code'] == parent) return;
    if (node['has_children'] == true) {
      await choose(
        node['code'],
        selected: (node['level'] as num) >= 2 ? node : null,
      );
    }
  }

  Future<void> manualEntry() async {
    manual.text = (widget.data['region_name'] ?? '').toString();
    final value = await showDialog<String>(
      context: context,
      builder: (c) => AlertDialog(
        title: Text(syncText(c, '历史／未覆盖地区', 'Historical or unlisted region')),
        content: NativeFieldLabel(
          label: ModelScope.of(c).t('regionName'),
          child: TextField(controller: manual, decoration: InputDecoration()),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(c),
            child: Text(ModelScope.of(c).t('cancel')),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(c, manual.text),
            child: Text(ModelScope.of(c).t('save')),
          ),
        ],
      ),
    );
    if (value != null && mounted) {
      widget.onChanged({
        'region_code': '',
        'region_name': value,
        'region_id': 0,
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final zh = ModelScope.of(context).language == 'zh',
        text = (widget.data['region_name'] ?? '').toString();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        OutlinedButton.icon(
          onPressed: !widget.enabled || loading || error != null
              ? null
              : () => choose(''),
          icon: const Icon(Icons.location_on_outlined),
          label: Text(
            text.isEmpty
                ? (zh ? '选择省／市／区县' : 'Select province / city / district')
                : text,
          ),
        ),
        if (loading) const LinearProgressIndicator(),
        if (error != null)
          Wrap(
            children: [
              Text(
                error == 'unsupported'
                    ? (zh
                          ? '当前服务器未提供区划级联，可保留原地区文字。'
                          : 'This server does not support region selection; existing text is retained.')
                    : (zh ? '地区加载失败' : 'Region loading failed'),
              ),
              if (error != 'unsupported')
                TextButton(onPressed: load, child: Text(zh ? '重试' : 'Retry')),
            ],
          ),
        TextButton(
          onPressed: widget.enabled ? manualEntry : null,
          child: Text(
            zh
                ? '历史或未覆盖地区，保留文字'
                : 'Use text for a historical or unlisted region',
          ),
        ),
        Text(
          zh
              ? '2023年目录，城市级可不细分区县。'
              : '2023 catalogue; district is optional for city-level data.',
          style: const TextStyle(fontSize: 12),
        ),
      ],
    );
  }
}

List<String> nativeKeywordSuggestions(Json data, String category) {
  final candidates = [(data['name'] ?? '').toString(), category];
  for (final line in (data['parameters'] ?? '').toString().split('\n')) {
    if (RegExp(
      r'^(型号|规格|Model|Specification)\s*[:：]',
      caseSensitive: false,
    ).hasMatch(line.trim())) {
      candidates.add(line.split(RegExp('[:：]')).skip(1).join(':').trim());
    }
  }
  final existing = (data['keywords'] ?? '')
      .toString()
      .split(RegExp('[,，;；、\n]'))
      .map((v) => v.trim())
      .toSet();
  return candidates
      .map((v) => v.trim())
      .where(
        (v) => v.isNotEmpty && v.runes.length <= 80 && !existing.contains(v),
      )
      .toSet()
      .toList();
}

String nativeParameterTemplate(String hall, bool zh) => hall == 'agriculture'
    ? (zh
          ? '型号：\n动力：\n幅宽：\n尺寸：\n重量：'
          : 'Model:\nPower:\nWorking width:\nDimensions:\nWeight:')
    : hall == 'cable'
    ? (zh
          ? '型号：\n规格：\n额定电压：\n导体截面积：'
          : 'Model:\nSpecification:\nRated voltage:\nConductor cross-section:')
    : (zh
          ? '型号：\n规格：\n尺寸：\n备注：'
          : 'Model:\nSpecification:\nDimensions:\nNotes:');

class UpgradeNotice extends StatelessWidget {
  final String text;
  const UpgradeNotice(this.text, {super.key});
  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.all(12),
    margin: const EdgeInsets.symmetric(vertical: 12),
    decoration: BoxDecoration(
      color: const Color(0xffeaf2ff),
      borderRadius: BorderRadius.circular(10),
    ),
    child: Text(text),
  );
}
