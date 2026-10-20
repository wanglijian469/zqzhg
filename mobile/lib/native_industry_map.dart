part of 'main.dart';

String nativeIndustryVendorPath(String query, int page, bool unassigned) =>
    '/public/industry-map/vendors?$query&page=$page&page_size=8&unassigned=${unassigned ? '1' : '0'}';

/// Only the verified public manifest is used. The local DataV preview API is
/// deliberately never consulted by the installed application.
class NativeBoundaryStore {
  final AppModel model;
  final Directory? directory;
  NativeBoundaryStore(this.model, {this.directory});
  final memory = <String, Json>{};
  Future<List<int>> read(String url) async {
    final source = model.baseUrl, uri = Uri.parse(source).resolve(url);
    if (uri.origin != Uri.parse(source).origin ||
        !RegExp(r'^/maps/[a-zA-Z0-9/_-]+\.json$').hasMatch(uri.path) ||
        uri.path.contains('..') ||
        uri.hasQuery ||
        uri.hasFragment) {
      throw ApiFailure('invalid_request');
    }
    final request = http.Request('GET', uri)..followRedirects = false;
    final response = await model.client
        .send(request)
        .timeout(const Duration(seconds: 15));
    if (response.statusCode != 200) {
      throw ApiFailure('not_found', const [], response.statusCode);
    }
    final bytes = <int>[];
    await for (final part in response.stream.timeout(
      const Duration(seconds: 15),
    )) {
      bytes.addAll(part);
      if (bytes.length > 12 * 1024 * 1024) throw ApiFailure('invalid_request');
    }
    if (model.baseUrl != source) throw ApiFailure('network_error');
    return bytes;
  }

  Future<Json?> load(String code, {bool refresh = false}) async {
    final source = model.baseUrl;
    final manifest = asMap(
      jsonDecode(utf8.decode(await read('/maps/manifest.json'))),
    );
    if (manifest['version'] != 1 ||
        manifest['catalog_version'] != '2023-06-30') {
      throw ApiFailure('invalid_request');
    }
    final value = asMap(
      manifest['resources'] ?? {},
    )[code.isEmpty ? 'national' : code];
    if (value == null) return null;
    final resource = asMap(value);
    final required = [
      'version',
      'source',
      'license',
      'review_number',
      'review_evidence',
      'publication_scope',
      'verified_by',
      'verified_at',
      'coordinate_system',
    ];
    if (resource['approved'] != true ||
        resource['presentation'] != 'industry-choropleth' ||
        resource['catalog_version'] != '2023-06-30' ||
        required.any(
          (key) => (resource[key] ?? '').toString().trim().isEmpty,
        ) ||
        DateTime.tryParse(resource['verified_at'] ?? '') == null ||
        !(resource['platforms'] as List? ?? []).contains('android') ||
        resource['cache_allowed'] != true ||
        ![
          'WGS84',
          'GCJ-02',
          'CGCS2000',
        ].contains(resource['coordinate_system']) ||
        !RegExp('^[a-f0-9]{64}\$').hasMatch(resource['sha256'] ?? '')) {
      throw ApiFailure('invalid_request');
    }
    final key = sha256
        .convert(utf8.encode('$source:${resource['sha256']}'))
        .toString();
    if (!refresh && memory.containsKey(key)) return memory[key];
    File? cached;
    try {
      final dir =
          directory ??
          Directory(
            '${(await getApplicationSupportDirectory()).path}/verified-boundaries',
          );
      await dir.create(recursive: true);
      cached = File('${dir.path}/$key.json');
    } catch (_) {
      /* Storage is optional; verification is not. */
    }
    Json verify(List<int> bytes) {
      if (sha256.convert(bytes).toString() != resource['sha256']) {
        throw ApiFailure('invalid_request');
      }
      final data = asMap(jsonDecode(utf8.decode(bytes)));
      if (data['type'] != 'FeatureCollection' ||
          rows(data['features']).isEmpty) {
        throw ApiFailure('invalid_request');
      }
      nativeGeoShapes(data);
      return data;
    }

    Json? data;
    if (!refresh && cached != null && await cached.exists()) {
      try {
        data = verify(await cached.readAsBytes());
      } catch (_) {
        await cached.delete();
      }
    }
    if (data == null) {
      final bytes = await read(resource['url']);
      data = verify(bytes);
      try {
        if (cached != null) {
          final temp = File('${cached.path}.tmp');
          await temp.writeAsBytes(bytes, flush: true);
          await temp.rename(cached.path);
          final files = await cached.parent
              .list()
              .where((f) => f is File && f.path.endsWith('.json'))
              .cast<File>()
              .toList();
          files.sort(
            (a, b) => a.statSync().modified.compareTo(b.statSync().modified),
          );
          for (final file in files.take(max(0, files.length - 24))) {
            await file.delete();
          }
        }
      } catch (_) {
        /* Continue with verified memory data. */
      }
    }
    if (source != model.baseUrl) throw ApiFailure('network_error');
    final map = asMap(resource['code_map'] ?? {}),
        field = resource['code_field'] ?? 'adcode';
    final features = rows(data['features']).map((feature) {
      final properties = asMap(feature['properties'] ?? {}),
          sourceCode = '${properties[field]}';
      return <String, dynamic>{
        ...feature,
        'properties': {...properties, 'adcode': map[sourceCode] ?? sourceCode},
      };
    }).toList();
    final result = <String, dynamic>{
      'resource': resource,
      'geometry': {...data, 'features': features},
    };
    memory[key] = result;
    if (memory.length > 12) memory.remove(memory.keys.first);
    return result;
  }
}

class NativeGeoShape {
  final String code, name;
  final Path path;
  NativeGeoShape(this.code, this.name, this.path);
}

List<NativeGeoShape> nativeGeoShapes(Json geometry) {
  final result = <NativeGeoShape>[];
  for (final f in rows(geometry['features'])) {
    final g = asMap(f['geometry'] ?? {}), p = asMap(f['properties'] ?? {});
    if (!['Polygon', 'MultiPolygon'].contains(g['type'])) {
      throw ApiFailure('invalid_request');
    }
    final polygons = g['type'] == 'Polygon'
        ? [g['coordinates']]
        : g['coordinates'] as List;
    final path = Path()..fillType = PathFillType.evenOdd;
    for (final polygon in polygons) {
      for (final ring in polygon as List) {
        final points = ring as List;
        if (points.length < 4) throw ApiFailure('invalid_request');
        for (var i = 0; i < points.length; i++) {
          final point = points[i] as List;
          final lon = (point[0] as num).toDouble(),
              lat = (point[1] as num).toDouble();
          if (!lon.isFinite ||
              !lat.isFinite ||
              lon.abs() > 180 ||
              lat.abs() > 90) {
            throw ApiFailure('invalid_request');
          }
          final x = lon * pi / 180,
              y = -log(tan(pi / 4 + lat.clamp(-85.0, 85.0) * pi / 360));
          if (i == 0) {
            path.moveTo(x, y);
          } else {
            path.lineTo(x, y);
          }
        }
        path.close();
      }
    }
    result.add(
      NativeGeoShape(
        '${p['adcode']}',
        (p['name'] ?? p['adcode']).toString(),
        path,
      ),
    );
  }
  return result;
}

class NativeOutlineMap extends StatefulWidget {
  final Json geometry;
  final Map<String, int> counts;
  final ValueChanged<String> onSelect;
  final String regionCode;
  const NativeOutlineMap({
    super.key,
    required this.geometry,
    required this.counts,
    required this.onSelect,
    required this.regionCode,
  });
  @override
  State<NativeOutlineMap> createState() => _NativeOutlineMapState();
}

class _NativeOutlineMapState extends State<NativeOutlineMap> {
  final transform = TransformationController();
  late List<NativeGeoShape> shapes;
  @override
  void initState() {
    super.initState();
    shapes = nativeGeoShapes(widget.geometry);
  }

  @override
  void didUpdateWidget(NativeOutlineMap old) {
    super.didUpdateWidget(old);
    if (old.geometry != widget.geometry) {
      shapes = nativeGeoShapes(widget.geometry);
    }
    if (old.regionCode != widget.regionCode) {
      transform.value = Matrix4.identity();
    }
  }

  @override
  void dispose() {
    transform.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, c) {
      final size = Size(c.maxWidth, c.maxHeight);
      final bounds = shapes
          .map((s) => s.path.getBounds())
          .reduce((a, b) => a.expandToInclude(b));
      final scale = min(
        (size.width - 32) / max(bounds.width, .000001),
        (size.height - 40) / max(bounds.height, .000001),
      );
      final origin = Offset(
        (size.width - bounds.width * scale) / 2 - bounds.left * scale,
        (size.height - bounds.height * scale) / 2 - bounds.top * scale,
      );
      return Stack(
        children: [
          InteractiveViewer(
            transformationController: transform,
            minScale: 1,
            maxScale: 12,
            child: GestureDetector(
              onTapUp: (details) {
                final point = (details.localPosition - origin) / scale;
                for (final s in shapes.reversed) {
                  if (s.path.contains(point)) {
                    if (widget.counts.containsKey(s.code)) {
                      widget.onSelect(s.code);
                    } else {
                      ScaffoldMessenger.of(context).showSnackBar(
                        SnackBar(
                          content: Text(
                            syncText(
                              context,
                              '未纳入本版统计',
                              'Outside this statistical catalogue',
                            ),
                          ),
                        ),
                      );
                    }
                    break;
                  }
                }
              },
              child: CustomPaint(
                size: size,
                painter: _NativeOutlinePainter(
                  shapes,
                  widget.counts,
                  origin,
                  scale,
                  ModelScope.of(context).language == 'zh',
                ),
              ),
            ),
          ),
          Positioned(
            right: 8,
            bottom: 8,
            child: IconButton.filledTonal(
              tooltip: syncText(context, '复位地图', 'Reset map'),
              onPressed: () => transform.value = Matrix4.identity(),
              icon: const Icon(Icons.center_focus_strong),
            ),
          ),
        ],
      );
    },
  );
}

class _NativeOutlinePainter extends CustomPainter {
  final List<NativeGeoShape> shapes;
  final Map<String, int> counts;
  final Offset origin;
  final double scale;
  final bool zh;
  _NativeOutlinePainter(
    this.shapes,
    this.counts,
    this.origin,
    this.scale,
    this.zh,
  );
  @override
  void paint(Canvas canvas, Size size) {
    final highest = counts.values.fold<int>(0, max);
    final labels = <Rect>[];
    for (final shape in shapes) {
      final path = shape.path.transform(
        (Matrix4.identity()
              ..translateByDouble(origin.dx, origin.dy, 0, 1)
              ..scaleByDouble(scale, scale, 1, 1))
            .storage,
      );
      final count = counts[shape.code];
      final fill = count == null
          ? const Color(0xffe6ebf0)
          : Color.lerp(
              const Color(0xffedf5ff),
              industrialBlue,
              highest == 0 ? 0 : (count / highest) * .85,
            )!;
      canvas.drawPath(path, Paint()..color = fill);
      canvas.drawPath(
        path,
        Paint()
          ..style = PaintingStyle.stroke
          ..strokeWidth = .7
          ..color = Colors.white,
      );
      final box = path.getBounds();
      if (box.width < 35 || box.height < 25) continue;
      final text = TextPainter(
        text: TextSpan(
          text: '${shape.name}${count == null ? '' : '\n$count'}',
          style: TextStyle(
            fontSize: 11,
            color: count != null && highest > 0 && count / highest > .65
                ? Colors.white
                : industrialInk,
          ),
        ),
        textDirection: TextDirection.ltr,
        textAlign: TextAlign.center,
        maxLines: 2,
        ellipsis: '…',
      )..layout(maxWidth: min(box.width, 100));
      final rect = Rect.fromCenter(
        center: box.center,
        width: text.width,
        height: text.height,
      );
      if (labels.any((r) => r.overlaps(rect))) continue;
      labels.add(rect);
      text.paint(canvas, rect.topLeft);
    }
  }

  @override
  bool shouldRepaint(_NativeOutlinePainter old) =>
      old.shapes != shapes ||
      old.counts != counts ||
      old.scale != scale ||
      old.origin != origin;
}

class NativeIndustryMapScreen extends StatefulWidget {
  const NativeIndustryMapScreen({super.key});
  @override
  State<NativeIndustryMapScreen> createState() =>
      _NativeIndustryMapScreenState();
}

class _NativeIndustryMapScreenState extends State<NativeIndustryMapScreen> {
  final search = TextEditingController();
  String code = '', q = '', scope = '', error = '';
  int? category;
  int page = 1, generation = 0;
  bool unassigned = false, list = false, loading = true;
  Json? statistics, vendors, boundary;
  List<Json> categories = [];
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    final next = '${m.baseUrl}:${m.activeHall.slug}';
    if (next != scope) {
      scope = next;
      code = q = '';
      category = null;
      page = 1;
      search.clear();
      unassigned = false;
      statistics = vendors = boundary = null;
      unawaited(load());
    }
  }

  @override
  void dispose() {
    search.dispose();
    generation++;
    super.dispose();
  }

  String params() => Uri(
    queryParameters: {
      'hall': ModelScope.of(context).activeHall.slug,
      if (code.isNotEmpty) 'region_code': code,
      if (q.isNotEmpty) 'q': q,
      if (category != null) 'category_id': '$category',
    },
  ).query;
  Future<void> load() async {
    final m = ModelScope.of(context), ticket = ++generation, source = scope;
    setState(() {
      loading = true;
      error = '';
    });
    try {
      final caps = await m.clientCapabilities();
      if (caps['industry_map'] != true) {
        throw ApiFailure('mobileUpgradeRequired');
      }
      final p = params();
      final results = await Future.wait([
        m.api('/public/industry-map/regions?$p'),
        m.api(nativeIndustryVendorPath(p, page, unassigned)),
        m.publicTaxonomy(),
      ]);
      if (!mounted || ticket != generation || source != scope) return;
      setState(() {
        statistics = asMap(results[0]);
        vendors = asMap(results[1]);
        categories = rows(results[2]['categories']);
        loading = false;
      });
      try {
        final resource = await m.boundaryStore.load(code);
        if (mounted && ticket == generation && source == scope) {
          setState(() => boundary = resource);
        }
      } catch (_) {
        if (mounted && ticket == generation) {
          setState(() => boundary = {'failed': true});
        }
      }
    } catch (e) {
      if (mounted && ticket == generation && source == scope) {
        setState(() {
          loading = false;
          error = e is ApiFailure ? e.code : 'network_error';
        });
      }
    }
  }

  void select(String next) {
    setState(() {
      code = next;
      page = 1;
      unassigned = false;
      boundary = null;
    });
    unawaited(load());
  }

  Future<void> choose() async {
    final items = rows(statistics?['items']);
    if (items.isEmpty) return;
    final node = await showModalBottomSheet<Json>(
      context: context,
      showDragHandle: true,
      builder: (c) => SafeArea(
        child: ListView(
          children: [
            for (final item in items)
              ListTile(
                title: Text(item['name']),
                trailing: Text('${item['vendor_count']}'),
                onTap: () => Navigator.pop(c, item),
              ),
          ],
        ),
      ),
    );
    if (node != null && mounted) select(node['code']);
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context),
        zh = m.language == 'zh',
        stats = statistics;
    final trail = rows(stats?['trail']);
    return PopScope(
      canPop: code.isEmpty,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) {
          select(trail.length > 1 ? trail[trail.length - 2]['code'] : '');
        }
      },
      child: Scaffold(
        appBar: appBar(context, zh ? '厂商地图' : 'Manufacturer map'),
        body: SafeArea(
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                child: Row(
                  children: [
                    Expanded(
                      child: TextField(
                        controller: search,
                        textInputAction: TextInputAction.search,
                        decoration: InputDecoration(
                          labelText: zh ? '搜索厂商或主营产品' : 'Search manufacturers',
                          suffixIcon: IconButton(
                            tooltip: m.t('search'),
                            icon: const Icon(Icons.search),
                            onPressed: () {
                              q = search.text.trim();
                              page = 1;
                              unawaited(load());
                            },
                          ),
                        ),
                        onSubmitted: (value) {
                          q = value.trim();
                          page = 1;
                          unawaited(load());
                        },
                      ),
                    ),
                    IconButton(
                      tooltip: zh ? '行业筛选' : 'Industry filter',
                      icon: const Icon(Icons.filter_alt_outlined),
                      onPressed: () async {
                        final id = await showModalBottomSheet<int>(
                          context: context,
                          builder: (c) => SafeArea(
                            child: ListView(
                              children: [
                                ListTile(
                                  title: Text(m.t('allCategories')),
                                  onTap: () => Navigator.pop(c, 0),
                                ),
                                for (final item in categories)
                                  ListTile(
                                    title: Text(m.content(item, 'name')),
                                    onTap: () => Navigator.pop(c, item['id']),
                                  ),
                              ],
                            ),
                          ),
                        );
                        if (id != null && mounted) {
                          setState(() {
                            category = id == 0 ? null : id;
                            page = 1;
                          });
                          unawaited(load());
                        }
                      },
                    ),
                  ],
                ),
              ),
              SingleChildScrollView(
                scrollDirection: Axis.horizontal,
                child: Row(
                  children: [
                    TextButton(
                      onPressed: () => select(''),
                      child: Text(zh ? '全国' : 'Nationwide'),
                    ),
                    for (final item in trail)
                      TextButton(
                        onPressed: () => select(item['code']),
                        child: Text(item['name']),
                      ),
                  ],
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 12),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        '${stats?['total'] ?? '—'} ${zh ? '家厂商' : 'manufacturers'}',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontWeight: FontWeight.bold),
                      ),
                    ),
                    IconButton(
                      tooltip: zh ? '选择地区' : 'Choose region',
                      onPressed: loading ? null : choose,
                      icon: const Icon(Icons.location_on_outlined),
                    ),
                    IconButton(
                      tooltip: zh ? '地图' : 'Map',
                      isSelected: !list,
                      onPressed: () => setState(() => list = false),
                      icon: const Icon(Icons.map_outlined),
                    ),
                    IconButton(
                      tooltip: zh ? '厂商列表' : 'Manufacturer list',
                      isSelected: list,
                      onPressed: () => setState(() => list = true),
                      icon: const Icon(Icons.view_list_outlined),
                    ),
                  ],
                ),
              ),
              if (loading) const LinearProgressIndicator(),
              if (error.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.all(12),
                  child: Column(
                    children: [
                      Text(m.t(error)),
                      TextButton(onPressed: load, child: Text(m.t('retry'))),
                    ],
                  ),
                ),
              if (stats != null && (stats['unassigned_count'] as num? ?? 0) > 0)
                TextButton(
                  onPressed: () {
                    setState(() {
                      unassigned = !unassigned;
                      page = 1;
                      list = true;
                    });
                    unawaited(load());
                  },
                  child: Text(
                    '${unassigned ? (zh ? '全部厂商' : 'All manufacturers') : (zh ? '未细分／待完善地区' : 'Unassigned region')} ${stats['unassigned_count']}',
                  ),
                ),
              Expanded(
                child: list
                    ? ListView(
                        padding: const EdgeInsets.all(12),
                        children: [
                          if (rows(vendors?['items']).isEmpty && !loading)
                            Text(
                              zh
                                  ? '当前条件下暂无公开厂商'
                                  : 'No public manufacturers match',
                            ),
                          for (final item in rows(vendors?['items']))
                            Card(
                              child: ListTile(
                                leading: SizedBox(
                                  width: 48,
                                  height: 48,
                                  child: Photo(
                                    id:
                                        asMap(
                                          asMap(
                                                item['data'],
                                              )['company_profile'] ??
                                              {},
                                        )['logo_media_id'] ??
                                        0,
                                    height: 48,
                                  ),
                                ),
                                title: Text(
                                  m.content(asMap(item['data']), 'name'),
                                ),
                                subtitle: Text(
                                  '${asMap(item['data'])['main_products'] ?? ''}\n${asMap(item['data'])['region_name'] ?? ''}',
                                  maxLines: 3,
                                ),
                                trailing: const Icon(Icons.chevron_right),
                                onTap: () => push(
                                  context,
                                  CompanyShowroomScreen(
                                    slug:
                                        item['showroom_slug'] ??
                                        'v-${item['id']}',
                                  ),
                                ),
                              ),
                            ),
                        ],
                      )
                    : boundary?['geometry'] != null
                    ? Column(
                        children: [
                          Expanded(
                            child: NativeOutlineMap(
                              regionCode: code,
                              geometry: asMap(boundary!['geometry']),
                              counts: {
                                for (final item in rows(stats?['items']))
                                  item['code']: item['vendor_count'],
                                if (rows(stats?['items']).isEmpty &&
                                    code.isNotEmpty)
                                  code: stats?['total'] ?? 0,
                              },
                              onSelect: (value) {
                                if (value != code) {
                                  select(value);
                                } else {
                                  ScaffoldMessenger.of(context).showSnackBar(
                                    SnackBar(
                                      content: Text(
                                        zh
                                            ? '已到最下级'
                                            : 'Lowest administrative level',
                                      ),
                                    ),
                                  );
                                }
                              },
                            ),
                          ),
                          Text(
                            '${asMap(boundary!['resource'])['source']} · ${asMap(boundary!['resource'])['review_number']}',
                            style: const TextStyle(fontSize: 10),
                          ),
                        ],
                      )
                    : SingleChildScrollView(
                        child: Padding(
                          padding: const EdgeInsets.all(24),
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              const Icon(
                                Icons.map_outlined,
                                size: 42,
                                color: industrialBlue,
                              ),
                              space(),
                              Text(
                                boundary?['failed'] == true
                                    ? (zh
                                          ? '轮廓加载失败，可重试。'
                                          : 'Boundary loading failed; retry.')
                                    : (zh
                                          ? '正式真实轮廓尚未开放。可选择地区查看统计与厂商，地图功能未完成验收。'
                                          : 'Verified outlines are not yet available. Browse regions and manufacturers; the map is not accepted as complete.'),
                                textAlign: TextAlign.center,
                              ),
                              TextButton(
                                onPressed: choose,
                                child: Text(zh ? '选择地区' : 'Choose region'),
                              ),
                              TextButton(
                                onPressed: load,
                                child: Text(m.t('retry')),
                              ),
                            ],
                          ),
                        ),
                      ),
              ),
              if (list)
                Padding(
                  padding: const EdgeInsets.all(8),
                  child: Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      IconButton(
                        tooltip: m.t('previous'),
                        onPressed: loading || page <= 1
                            ? null
                            : () {
                                page--;
                                unawaited(load());
                              },
                        icon: const Icon(Icons.chevron_left),
                      ),
                      Text(
                        '$page / ${max(1, ((vendors?['total'] as num? ?? 0) / 8).ceil())}',
                      ),
                      IconButton(
                        tooltip: m.t('next'),
                        onPressed:
                            loading ||
                                page * 8 >= (vendors?['total'] as num? ?? 0)
                            ? null
                            : () {
                                page++;
                                unawaited(load());
                              },
                        icon: const Icon(Icons.chevron_right),
                      ),
                    ],
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
