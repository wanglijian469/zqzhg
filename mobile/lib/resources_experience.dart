part of 'main.dart';

String resourceTitle(AppModel m, Json r) =>
    m.language == 'en' && (r['data']['title_en'] ?? '').toString().isNotEmpty
    ? r['data']['title_en']
    : r['data']['title'];

class CompanyShowroomScreen extends StatefulWidget {
  final String slug;
  final int initialTab;
  const CompanyShowroomScreen({
    super.key,
    required this.slug,
    this.initialTab = 5,
  });
  @override
  State<CompanyShowroomScreen> createState() => _CompanyShowroomScreenState();
}

class _CompanyShowroomScreenState extends State<CompanyShowroomScreen> {
  late int tab = widget.initialTab;
  int page = 1;
  String query = '';
  int category = 0;
  final search = TextEditingController();
  @override
  void dispose() {
    search.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      appBar: appBar(context, m.t('showroom')),
      body: AsyncPane(
        identity: '${m.baseUrl}:showroom:${widget.slug}',
        load: () =>
            m.api('/public/showrooms/${Uri.encodeComponent(widget.slug)}'),
        builder: (result) {
          final item = asMap(result['item']);
          final data = asMap(item['data']);
          final profile = asMap(data['company_profile'] ?? {});
          final cats = rows(result['categories']);
          final counts = asMap(result['resource_counts'] ?? {});
          final available = [
            5,
            0,
            if ((result['product_count'] ?? 0) > 0) 1,
            if ((counts['video'] ?? 0) > 0) 2,
            if ((counts['catalog'] ?? 0) > 0) 3,
            4,
          ];
          if (!available.contains(tab)) tab = 5;
          return NestedScrollView(
            headerSliverBuilder: (context, scrolled) => [
              SliverToBoxAdapter(
                child: NativeBrandHero(
                  favoriteAction: FavoriteButton(
                    item: {...item, 'kind': 'vendor'},
                    onDark: true,
                  ),
                  data: data,
                  onContact: () => setState(() => tab = 4),
                  onShare: () =>
                      push(context, ShowroomPromotion(slug: widget.slug)),
                ),
              ),
              SliverPersistentHeader(
                pinned: true,
                delegate: ShowroomTabsHeader(
                  height: 64 * MediaQuery.textScalerOf(context).scale(12) / 12,
                  child: SingleChildScrollView(
                    scrollDirection: Axis.horizontal,
                    child: Row(
                      children: [
                        for (final i in available)
                          Padding(
                            padding: const EdgeInsets.symmetric(horizontal: 4),
                            child: ChoiceChip(
                              label: Text(
                                m.t(
                                  [
                                    'companyIntro',
                                    'exhibits',
                                    'shortVideos',
                                    'electronicCatalogs',
                                    'showroomContact',
                                    'portalHome',
                                  ][i],
                                ),
                              ),
                              selected: tab == i,
                              onSelected: (_) => setState(() => tab = i),
                            ),
                          ),
                      ],
                    ),
                  ),
                ),
              ),
            ],
            body: ListView(
              padding: const EdgeInsets.all(20),
              children: [
                if (tab == 5 || tab == 0)
                  CompanyPortalBody(
                    data: data,
                    result: result,
                    compact: tab == 5,
                    onTab: (v) => setState(() => tab = v),
                  ),
                if (tab == 1) ...[
                  if (rows(result['representative_products']).isNotEmpty) ...[
                    Text(
                      m.t('representativeProducts'),
                      style: Theme.of(context).textTheme.titleLarge,
                    ),
                    ExhibitGrid(items: rows(result['representative_products'])),
                  ],
                  TextField(
                    controller: search,
                    decoration: InputDecoration(
                      labelText: m.t('showroomKeyword'),
                      suffixIcon: IconButton(
                        icon: const Icon(Icons.search),
                        onPressed: () => setState(() {
                          query = search.text;
                          page = 1;
                        }),
                      ),
                    ),
                    onSubmitted: (v) => setState(() {
                      query = v;
                      page = 1;
                    }),
                  ),
                  space(),
                  DropdownButtonFormField<int>(
                    initialValue: category,
                    isExpanded: true,
                    decoration: InputDecoration(labelText: m.t('category')),
                    items: [
                      DropdownMenuItem(
                        value: 0,
                        child: Text(m.t('allCategories')),
                      ),
                      for (final c in cats)
                        DropdownMenuItem(
                          value: c['id'],
                          child: Text(
                            '${m.content(c, 'name')} (${c['product_count'] ?? 0})',
                            overflow: TextOverflow.ellipsis,
                          ),
                        ),
                    ],
                    onChanged: (v) => setState(() {
                      category = v ?? 0;
                      page = 1;
                    }),
                  ),
                  space(),
                  AsyncPane(
                    identity:
                        '${m.baseUrl}:${item['id']}:$query:$category:$page',
                    load: () => m.api(
                      '/public/products?${Uri(queryParameters: {'vendor_id': '${item['id']}', 'q': query, 'category_id': category == 0 ? '' : '$category', 'page': '$page', 'page_size': '12'}).query}',
                    ),
                    builder: (products) => Column(
                      children: [
                        Text('${products['total']} ${m.t('productCount')}'),
                        ExhibitGrid(items: rows(products['items'])),
                        if (rows(products['items']).isEmpty)
                          message(context, m.t('noProductResults')),
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            TextButton(
                              onPressed: page > 1
                                  ? () => setState(() => page--)
                                  : null,
                              child: Text(m.t('previous')),
                            ),
                            Text('$page'),
                            TextButton(
                              onPressed: page * 12 < (products['total'] as num)
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
                if (tab == 2)
                  NativeResourceList(
                    key: const ValueKey('videos'),
                    vendorID: item['id'],
                    kind: 'video',
                  ),
                if (tab == 3)
                  NativeResourceList(
                    key: const ValueKey('catalogs'),
                    vendorID: item['id'],
                    kind: 'catalog',
                  ),
                if (tab == 4) ...[
                  Text(
                    m.t('showroomContact'),
                    style: const TextStyle(
                      fontSize: 22,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                  space(),
                  Contact(data: data),
                  if ((data['address'] ?? '').toString().isNotEmpty)
                    ListTile(
                      title: Text(m.t('companyAddress')),
                      subtitle: Text(data['address']),
                    ),
                  if ((profile['contact_name'] ?? '').toString().isNotEmpty)
                    ListTile(
                      title: Text(m.t('contactName')),
                      subtitle: Text(profile['contact_name']),
                    ),
                  if ((profile['website'] ?? '').toString().isNotEmpty)
                    TextButton(
                      onPressed: () => launchUrl(
                        Uri.parse(profile['website']),
                        mode: LaunchMode.externalApplication,
                      ),
                      child: Text(profile['website']),
                    ),
                  for (final c in rows(profile['contacts']))
                    Card(
                      child: Padding(
                        padding: const EdgeInsets.all(14),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('${c['area']} · ${c['name']}'),
                            Contact(data: c),
                          ],
                        ),
                      ),
                    ),
                ],
                space(24),
              ],
            ),
          );
        },
      ),
    );
  }
}

class NativeResourceList extends StatefulWidget {
  final int? vendorID, productID;
  final String? kind;
  final bool management;
  const NativeResourceList({
    super.key,
    this.vendorID,
    this.productID,
    this.kind,
    this.management = false,
  });
  @override
  State<NativeResourceList> createState() => _NativeResourceListState();
}

class _NativeResourceListState extends State<NativeResourceList> {
  int page = 1, version = 0;
  String purpose = '', status = '', query = '';
  Timer? pollTimer;
  @override
  void initState() {
    super.initState();
    if (widget.management) {
      pollTimer = Timer.periodic(const Duration(seconds: 3), (_) {
        if (mounted) setState(() => version++);
      });
    }
  }

  @override
  void dispose() {
    pollTimer?.cancel();
    super.dispose();
  }

  Future<void> refresh() async {
    if (mounted) setState(() => version++);
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final q = {
      'page': '$page',
      'page_size': '12',
      if (widget.vendorID != null) 'vendor_id': '${widget.vendorID}',
      if (widget.productID != null) 'product_id': '${widget.productID}',
      if (widget.kind != null) 'kind': widget.kind!,
      'purpose': purpose,
      'status': status,
      'q': query,
    };
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (widget.management) ...[
          FilledButton.icon(
            onPressed: () async {
              await Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) => NativeResourceEditor(
                    kind: widget.kind!,
                    productID: widget.productID,
                  ),
                ),
              );
              refresh();
            },
            icon: const Icon(Icons.add),
            label: Text(m.t('newResource')),
          ),
          space(),
          TextField(
            decoration: InputDecoration(labelText: m.t('keyword')),
            onChanged: (v) => setState(() {
              query = v;
              page = 1;
            }),
          ),
          space(),
          DropdownButtonFormField<String>(
            initialValue: status,
            isExpanded: true,
            decoration: InputDecoration(labelText: m.t('reviewStatus')),
            items: [
              DropdownMenuItem(value: '', child: Text(m.t('allStatus'))),
              for (final s in ['draft', 'pending', 'approved', 'rejected'])
                DropdownMenuItem(value: s, child: Text(m.t(s))),
            ],
            onChanged: (v) => setState(() {
              status = v ?? '';
              page = 1;
            }),
          ),
        ],
        if (!widget.management && widget.kind == 'video')
          DropdownButtonFormField<String>(
            initialValue: purpose,
            isExpanded: true,
            decoration: InputDecoration(labelText: m.t('purpose')),
            items: [
              DropdownMenuItem(value: '', child: Text(m.t('allVideoTypes'))),
              for (final p in [
                ['company', 'companyVideo'],
                ['production', 'productionVideo'],
                ['product', 'productVideo'],
              ])
                DropdownMenuItem(value: p[0], child: Text(m.t(p[1]))),
            ],
            onChanged: (v) => setState(() {
              purpose = v ?? '';
              page = 1;
            }),
          ),
        space(),
        AsyncPane(
          identity:
              '${m.scope}:${widget.management}:$version:${Uri(queryParameters: q).query}',
          load: () => m.api(
            '/${widget.management ? 'merchant' : 'public'}/resources?${Uri(queryParameters: q).query}',
          ),
          builder: (r) => Column(
            children: [
              if (rows(r['items']).isEmpty)
                message(context, m.t('noResources')),
              for (final item in rows(r['items']))
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(14),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (!widget.management)
                          InkWell(
                            onTap: () =>
                                push(context, NativeResourceViewer(item: item)),
                            child: item['data']['cover_media_id'] != 0
                                ? Photo(
                                    id: item['data']['cover_media_id'],
                                    height: 180,
                                  )
                                : item['kind'] == 'video'
                                ? Photo(
                                    id: item['data']['file_media_id'],
                                    height: 180,
                                  )
                                : const SizedBox(
                                    height: 100,
                                    child: Center(
                                      child: Icon(
                                        Icons.picture_as_pdf,
                                        size: 44,
                                      ),
                                    ),
                                  ),
                          ),
                        Text(
                          resourceTitle(m, item),
                          style: const TextStyle(
                            fontSize: 19,
                            fontWeight: FontWeight.bold,
                          ),
                        ),
                        if (item['kind'] == 'video' && item['file'] != null)
                          Text('${(item['file']['duration'] as num).ceil()} s')
                        else
                          Text(
                            '${item['file']?['page_count'] ?? (item['data']['page_media_ids'] as List).length} ${m.t('pageNumber')}',
                          ),
                        if (widget.management) ...[
                          Wrap(
                            spacing: 8,
                            children: [
                              Chip(label: Text(m.t(item['status']))),
                              Chip(
                                label: Text(
                                  m.t(
                                    item['public_visible'] == true
                                        ? 'published'
                                        : 'unpublished',
                                  ),
                                ),
                              ),
                              if (item['file'] != null)
                                Chip(label: Text(m.t(item['file']['state']))),
                            ],
                          ),
                          if ((item['review_note'] ?? '').toString().isNotEmpty)
                            message(
                              context,
                              item['review_note'] == 'offline'
                                  ? m.t('offline')
                                  : item['review_note'],
                            ),
                        ],
                        Wrap(
                          spacing: 12,
                          runSpacing: 8,
                          children: [
                            OutlinedButton(
                              onPressed: () => push(
                                context,
                                NativeResourceViewer(
                                  item: item,
                                  private: widget.management,
                                ),
                              ),
                              child: Text(
                                m.t(
                                  widget.management
                                      ? 'preview'
                                      : item['kind'] == 'video'
                                      ? 'play'
                                      : 'readCatalog',
                                ),
                              ),
                            ),
                            if (widget.management) ...[
                              OutlinedButton(
                                onPressed: () async {
                                  await Navigator.push(
                                    context,
                                    MaterialPageRoute(
                                      builder: (_) => NativeResourceEditor(
                                        kind: item['kind'],
                                        item: item,
                                        productID: widget.productID,
                                      ),
                                    ),
                                  );
                                  refresh();
                                },
                                child: Text(m.t('edit')),
                              ),
                              if (item['status'] == 'draft')
                                FilledButton(
                                  onPressed:
                                      item['file'] != null &&
                                          item['file']['state'] != 'ready'
                                      ? null
                                      : () async {
                                          try {
                                            await m.api(
                                              '/merchant/resources/${item['id']}/submit',
                                              method: 'POST',
                                              data: {
                                                'revision_id':
                                                    item['revision_id'],
                                              },
                                            );
                                            refresh();
                                          } catch (e) {
                                            if (context.mounted) {
                                              showError(context, e);
                                            }
                                          }
                                        },
                                  child: Text(m.t('submit')),
                                ),
                              if (item['visible'] == true)
                                TextButton(
                                  onPressed: () async {
                                    final ok = await showDialog<bool>(
                                      context: context,
                                      builder: (c) => AlertDialog(
                                        content: Text(m.t('offlineConfirm')),
                                        actions: [
                                          TextButton(
                                            onPressed: () =>
                                                Navigator.pop(c, false),
                                            child: Text(m.t('cancel')),
                                          ),
                                          TextButton(
                                            onPressed: () =>
                                                Navigator.pop(c, true),
                                            child: Text(m.t('offline')),
                                          ),
                                        ],
                                      ),
                                    );
                                    if (ok != true) return;
                                    try {
                                      await m.api(
                                        '/merchant/resources/${item['id']}/offline',
                                        method: 'POST',
                                        data: {},
                                      );
                                      refresh();
                                    } catch (e) {
                                      if (context.mounted) {
                                        showError(context, e);
                                      }
                                    }
                                  },
                                  child: Text(m.t('offline')),
                                ),
                              TextButton(
                                onPressed: () => push(
                                  context,
                                  ResourceHistoryScreen(id: item['id']),
                                ),
                                child: Text(m.t('history')),
                              ),
                            ],
                          ],
                        ),
                      ],
                    ),
                  ),
                ),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  TextButton(
                    onPressed: page > 1 ? () => setState(() => page--) : null,
                    child: Text(m.t('previous')),
                  ),
                  Text('$page'),
                  TextButton(
                    onPressed: page * 12 < (r['total'] as num)
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
    );
  }
}

class ResourceHistoryScreen extends StatelessWidget {
  final int id;
  const ResourceHistoryScreen({super.key, required this.id});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Scaffold(
      appBar: appBar(context, m.t('history')),
      body: AsyncPane(
        identity: '${m.scope}:resource-history:$id',
        load: () => m.api('/merchant/resources/$id/history'),
        builder: (r) => ListView(
          children: [
            for (final log in rows(r))
              ListTile(
                title: Text(m.t(log['action'])),
                subtitle: Text('${log['created_at']}\n${log['note']}'),
              ),
          ],
        ),
      ),
    );
  }
}

class NativeResourceViewer extends StatefulWidget {
  final Json item;
  final bool private;
  const NativeResourceViewer({
    super.key,
    required this.item,
    this.private = false,
  });
  @override
  State<NativeResourceViewer> createState() => _NativeResourceViewerState();
}

class _NativeResourceViewerState extends State<NativeResourceViewer>
    with WidgetsBindingObserver {
  VideoPlayerController? video;
  String error = '';
  int page = 0;
  final pdfController = PdfViewerController();
  bool loading = false;
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (widget.item['kind'] == 'video' &&
        video == null &&
        (widget.item['file']?['state'] == null ||
            widget.item['file']['state'] == 'ready')) {
      final m = ModelScope.of(context);
      video = VideoPlayerController.networkUrl(
        Uri.parse(
          '${m.baseUrl}/api/v1/media/${widget.item['data']['file_media_id']}',
        ),
        httpHeaders: widget.private
            ? {'Authorization': 'Bearer ${m.token}'}
            : {},
      );
      video!
          .initialize()
          .then((_) {
            if (mounted) setState(() {});
          })
          .catchError((e) {
            if (mounted) setState(() => error = 'network_error');
          });
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state != AppLifecycleState.resumed) {
      video?.pause();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    video?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final d = asMap(widget.item['data']);
    return Scaffold(
      appBar: appBar(context, resourceTitle(m, widget.item)),
      body: Column(
        children: [
          if (error.isNotEmpty) message(context, m.t(error)),
          Expanded(
            child:
                widget.item['file']?['state'] != null &&
                    widget.item['file']['state'] != 'ready'
                ? Center(child: Text(m.t('noPreview')))
                : widget.item['kind'] == 'video'
                ? Center(
                    child: video?.value.isInitialized == true
                        ? Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              AspectRatio(
                                aspectRatio: video!.value.aspectRatio,
                                child: VideoPlayer(video!),
                              ),
                              VideoProgressIndicator(
                                video!,
                                allowScrubbing: true,
                                padding: const EdgeInsets.all(12),
                              ),
                              FilledButton.icon(
                                onPressed: () {
                                  setState(() {
                                    video!.value.isPlaying
                                        ? video!.pause()
                                        : video!.play();
                                  });
                                },
                                icon: Icon(
                                  video!.value.isPlaying
                                      ? Icons.pause
                                      : Icons.play_arrow,
                                ),
                                label: Text(m.t('play')),
                              ),
                            ],
                          )
                        : error.isEmpty
                        ? const CircularProgressIndicator()
                        : const SizedBox(),
                  )
                : d['format'] == 'pdf'
                ? PdfViewer.uri(
                    Uri.parse(
                      '${m.baseUrl}/api/v1/media/${d['file_media_id']}',
                    ),
                    controller: pdfController,
                    headers: widget.private
                        ? {'Authorization': 'Bearer ${m.token}'}
                        : {},
                    preferRangeAccess: true,
                  )
                : InteractiveViewer(
                    child: Image.network(
                      '${m.baseUrl}/api/v1/media/${d['page_media_ids'][page]}',
                      headers: widget.private
                          ? {'Authorization': 'Bearer ${m.token}'}
                          : {},
                      fit: BoxFit.contain,
                      errorBuilder: (_, _, _) => Text(m.t('network_error')),
                    ),
                  ),
          ),
          if (widget.item['kind'] == 'catalog')
            Wrap(
              spacing: 8,
              alignment: WrapAlignment.center,
              children: [
                if (d['format'] == 'images') ...[
                  TextButton(
                    onPressed: page > 0 ? () => setState(() => page--) : null,
                    child: Text(m.t('previous')),
                  ),
                  Text('${page + 1} / ${(d['page_media_ids'] as List).length}'),
                  TextButton(
                    onPressed: page + 1 < (d['page_media_ids'] as List).length
                        ? () => setState(() => page++)
                        : null,
                    child: Text(m.t('next')),
                  ),
                ] else ...[
                  TextButton(
                    onPressed: () => pdfController.zoomDown(),
                    child: Text(m.t('zoomOut')),
                  ),
                  TextButton(
                    onPressed: () => pdfController.zoomUp(),
                    child: Text(m.t('zoomIn')),
                  ),
                  TextButton(
                    onPressed: loading
                        ? null
                        : () async {
                            setState(() => loading = true);
                            try {
                              final response = await m.client.get(
                                Uri.parse(
                                  '${m.baseUrl}/api/v1/media/${d['file_media_id']}',
                                ),
                                headers: widget.private
                                    ? {'Authorization': 'Bearer ${m.token}'}
                                    : {},
                              );
                              if (response.statusCode != 200) {
                                throw ApiFailure('network_error');
                              }
                              await sharingBridge.invokeMethod('saveFile', {
                                'bytes': response.bodyBytes,
                                'name': '${d['title']}.pdf',
                                'mime': 'application/pdf',
                              });
                            } catch (e) {
                              if (context.mounted) showError(context, e);
                            } finally {
                              if (mounted) setState(() => loading = false);
                            }
                          },
                    child: Text(m.t('downloadPDF')),
                  ),
                ],
              ],
            ),
          if ((d['description'] ?? '').toString().isNotEmpty)
            Padding(
              padding: const EdgeInsets.all(12),
              child: Text(
                m.language == 'en' &&
                        (d['description_en'] ?? '').toString().isNotEmpty
                    ? d['description_en']
                    : d['description'],
              ),
            ),
          if (d['product_id'] != 0 && !widget.private)
            TextButton(
              onPressed: () => push(
                context,
                DetailScreen(id: d['product_id'], kind: 'product'),
              ),
              child: Text(m.t('viewProduct')),
            ),
          space(12),
        ],
      ),
    );
  }
}

class ProgressMultipart extends http.MultipartRequest {
  final void Function(int, int) progress;
  ProgressMultipart(super.method, super.url, this.progress);
  @override
  http.ByteStream finalize() {
    final stream = super.finalize();
    final total = contentLength;
    var sent = 0;
    return http.ByteStream(
      stream.map((chunk) {
        sent += chunk.length;
        progress(sent, total);
        return chunk;
      }),
    );
  }
}

class NativeResourceEditor extends StatefulWidget {
  final String kind;
  final Json? item;
  final int? productID;
  const NativeResourceEditor({
    super.key,
    required this.kind,
    this.item,
    this.productID,
  });
  @override
  State<NativeResourceEditor> createState() => _NativeResourceEditorState();
}

class _NativeResourceEditorState extends State<NativeResourceEditor> {
  Json data = {}, metadata = {};
  Json? current;
  List<Json> uploads = [];
  final fields = <String, TextEditingController>{};
  bool initialized = false, busy = false, uploading = false, cancelled = false;
  int progress = 0;
  String? draftKey;
  Timer? timer;
  http.Client? uploadClient;
  int ownerID = 0;
  String sourceEndpoint = '';
  bool retired = false;
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final m = ModelScope.of(context);
    if (initialized) {
      if (!retired &&
          (m.baseUrl != sourceEndpoint ||
              (ownerID != 0 &&
                  m.token.isNotEmpty &&
                  m.account['id'] != ownerID))) {
        retired = true;
        cancelled = true;
        timer?.cancel();
        uploadClient?.close();
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) {
            returnToMerchant(context);
          }
        });
      }
      return;
    }
    initialized = true;
    ownerID = m.account['id'] ?? 0;
    sourceEndpoint = m.baseUrl;
    current = widget.item;
    draftKey =
        'resource.draft.${m.scope}.${widget.item?['id'] ?? '${widget.kind}.${widget.productID ?? 0}'}';
    data = widget.item == null
        ? {
            'title': '',
            'title_en': '',
            'description': '',
            'description_en': '',
            'purpose': widget.productID != null && widget.kind == 'video'
                ? 'product'
                : 'company',
            'format': widget.kind == 'video' ? 'video' : 'pdf',
            'product_id': widget.productID ?? 0,
            'file_media_id': 0,
            'cover_media_id': 0,
            'page_media_ids': <int>[],
          }
        : asMap(widget.item!['data']);
    metadata = asMap(widget.item?['file'] ?? {});
    try {
      final saved = asMap(jsonDecode(m.prefs.getString(draftKey!) ?? '{}'));
      if (saved['revision_id'] == (current?['revision_id'] ?? 0)) {
        data = asMap(saved['data']);
        uploads = rows(saved['uploads']);
      }
    } catch (_) {}
    for (final k in ['title', 'title_en', 'description', 'description_en']) {
      fields[k] = TextEditingController(text: data[k]);
      fields[k]!.addListener(() {
        data[k] = fields[k]!.text;
        unawaited(persist());
      });
    }
    timer = Timer.periodic(
      const Duration(seconds: 2),
      (_) => unawaited(poll()),
    );
  }

  Future<void> persist() async {
    if (!mounted || draftKey == null || retired) return;
    await ModelScope.of(context).prefs.setString(
      draftKey!,
      jsonEncode({
        'revision_id': current?['revision_id'] ?? 0,
        'data': data,
        'uploads': uploads,
      }),
    );
  }

  Future<void> poll() async {
    if (!mounted || retired || data['file_media_id'] == 0) return;
    final m = ModelScope.of(context);
    try {
      final v = asMap(
        await m.api('/merchant/resource-media/${data['file_media_id']}'),
      );
      if (mounted) setState(() => metadata = v);
    } catch (_) {}
  }

  @override
  void dispose() {
    timer?.cancel();
    uploadClient?.close();
    for (final f in fields.values) {
      f.dispose();
    }
    super.dispose();
  }

  Future<void> pick(String kind) async {
    try {
      final result = await FilePicker.pickFiles(
        type: FileType.custom,
        allowedExtensions: kind == 'video'
            ? ['mp4', 'mov']
            : kind == 'pdf'
            ? ['pdf']
            : ['jpg', 'jpeg', 'png', 'webp'],
      );
      if (result.isEmpty || !mounted) return;
      final root = await getApplicationDocumentsDirectory();
      final dir = Directory('${root.path}/resource-uploads');
      await dir.create(recursive: true);
      final remaining =
          30 -
          (data['page_media_ids'] as List).length -
          uploads
              .where((f) => f['kind'] == 'pages' && f['media_id'] == null)
              .length;
      if (kind == 'pages' && remaining <= 0) {
        throw ApiFailure('resource_limit_reached');
      }
      for (final f
          in (kind == 'pages' ? result.take(remaining) : result.take(1))) {
        final maxSize =
            (kind == 'video'
                ? 200
                : kind == 'pdf'
                ? 50
                : 10) *
            1024 *
            1024;
        if ((await f.length() ?? 0) > maxSize) {
          throw ApiFailure('file_too_large');
        }
        final ext = f.extension ?? 'bin';
        final path =
            '${dir.path}/${DateTime.now().microsecondsSinceEpoch}.$ext';
        var actual = 0;
        final sink = File(path).openWrite();
        try {
          await sink.addStream(
            f.readAsByteStream().map((chunk) {
              actual += chunk.length;
              if (actual > maxSize) throw ApiFailure('file_too_large');
              return chunk;
            }),
          );
        } finally {
          await sink.close();
        }
        if (actual == 0) throw ApiFailure('invalid_request');
        uploads.add({'path': path, 'name': f.name, 'kind': kind});
      }
      if (mounted) setState(() {});
      await persist();
    } catch (e) {
      if (mounted) showError(context, e);
    }
  }

  Future<void> upload() async {
    await persist();
    if (!mounted ||
        retired ||
        !await nativeSessionReady(context, ownerID, sourceEndpoint) ||
        !mounted ||
        retired) {
      return;
    }
    final m = ModelScope.of(context);
    setState(() {
      uploading = true;
      cancelled = false;
    });
    try {
      for (final file in uploads.where((f) => f['media_id'] == null)) {
        if (cancelled) break;
        uploadClient = http.Client();
        final path = file['kind'] == 'pages' || file['kind'] == 'cover'
            ? '/merchant/media'
            : '/merchant/resource-media?kind=${file['kind']}';
        final request = ProgressMultipart(
          'POST',
          Uri.parse('${m.baseUrl}/api/v1$path'),
          (sent, total) {
            final p = (sent / total * 100).floor();
            if (mounted && p != progress) setState(() => progress = p);
          },
        );
        request.headers['Authorization'] = 'Bearer ${m.token}';
        request.files.add(
          await http.MultipartFile.fromPath(
            'file',
            file['path'],
            filename: file['name'],
          ),
        );
        final response = await http.Response.fromStream(
          await uploadClient!
              .send(request)
              .timeout(const Duration(minutes: 15)),
        ).timeout(const Duration(minutes: 15));
        if (response.statusCode >= 400) {
          if (response.statusCode == 401) await m.expireSession();
          final error = asMap(jsonDecode(response.body));
          throw ApiFailure(error['error']?['code'] ?? 'network_error');
        }
        final result = asMap(jsonDecode(response.body));
        final id = result['media_id'] ?? result['id'];
        file['media_id'] = id;
        if (file['kind'] == 'pages') {
          data['page_media_ids'] = [...(data['page_media_ids'] as List), id];
        } else if (file['kind'] == 'cover') {
          data['cover_media_id'] = id;
        } else {
          data['file_media_id'] = id;
          metadata = result;
        }
        await persist();
        uploadClient!.close();
        uploadClient = null;
      }
    } catch (e) {
      if (mounted && !cancelled) showError(context, e);
    } finally {
      uploadClient?.close();
      uploadClient = null;
      if (mounted) setState(() => uploading = false);
    }
  }

  Future<void> save(bool submit) async {
    await persist();
    if (!mounted ||
        retired ||
        !await nativeSessionReady(context, ownerID, sourceEndpoint) ||
        !mounted ||
        retired) {
      return;
    }
    final m = ModelScope.of(context);
    setState(() => busy = true);
    try {
      final r = asMap(
        await m.api(
          current == null
              ? '/merchant/resources'
              : '/merchant/resources/${current!['id']}/draft',
          method: current == null ? 'POST' : 'PUT',
          data: current == null
              ? {'kind': widget.kind, 'data': data}
              : {'revision_id': current!['revision_id'], 'data': data},
        ),
      );
      current = r;
      data = asMap(r['data']);
      await persist();
      if (submit) {
        await m.api(
          '/merchant/resources/${r['id']}/submit',
          method: 'POST',
          data: {'revision_id': r['revision_id']},
        );
        await m.prefs.remove(draftKey!);
        if (mounted) Navigator.pop(context, true);
      } else if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(m.t('resourceSaved'))));
      }
    } catch (e) {
      if (mounted) showError(context, e);
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final disabled =
        retired || busy || uploading || current?['status'] == 'pending';
    return PopScope(
      onPopInvokedWithResult: (p, _) {
        if (p) unawaited(persist());
      },
      child: Scaffold(
        appBar: appBar(
          context,
          m.t(widget.kind == 'video' ? 'shortVideos' : 'electronicCatalogs'),
        ),
        body: ListView(
          padding: const EdgeInsets.all(20),
          children: [
            message(context, m.t('resourceLimit')),
            for (final k in ['title', 'description'])
              Padding(
                padding: const EdgeInsets.only(bottom: 18),
                child: TextField(
                  controller: fields[k],
                  enabled: !disabled,
                  maxLength: k == 'title' ? 200 : 10000,
                  maxLines: k == 'title' ? 1 : 3,
                  decoration: InputDecoration(
                    labelText: m.t(
                      k == 'title' ? 'resourceTitle' : 'resourceDescription',
                    ),
                  ),
                ),
              ),
            ExpansionTile(
              title: Text(m.t('englishInfo')),
              children: [
                TextField(
                  controller: fields['title_en'],
                  enabled: !disabled,
                  maxLength: 200,
                  decoration: InputDecoration(labelText: m.t('nameEN')),
                ),
                TextField(
                  controller: fields['description_en'],
                  enabled: !disabled,
                  maxLength: 10000,
                  maxLines: 3,
                  decoration: InputDecoration(
                    labelText: m.t('englishDescription'),
                  ),
                ),
              ],
            ),
            if (widget.kind == 'video')
              DropdownButtonFormField<String>(
                initialValue: data['purpose'],
                isExpanded: true,
                decoration: InputDecoration(labelText: m.t('purpose')),
                items: [
                  for (final p in [
                    ['company', 'companyVideo'],
                    ['production', 'productionVideo'],
                    ['product', 'productVideo'],
                  ])
                    DropdownMenuItem(value: p[0], child: Text(m.t(p[1]))),
                ],
                onChanged: disabled
                    ? null
                    : (v) {
                        setState(() => data['purpose'] = v);
                        persist();
                      },
              )
            else
              DropdownButtonFormField<String>(
                initialValue: data['format'],
                isExpanded: true,
                decoration: InputDecoration(labelText: m.t('catalogFormat')),
                items: [
                  DropdownMenuItem(value: 'pdf', child: Text(m.t('pdf'))),
                  DropdownMenuItem(value: 'images', child: Text(m.t('images'))),
                ],
                onChanged: disabled
                    ? null
                    : (v) {
                        setState(() {
                          data['format'] = v;
                          data['file_media_id'] = 0;
                          data['page_media_ids'] = <int>[];
                          metadata = {};
                          uploads = [];
                        });
                        persist();
                      },
              ),
            space(),
            Text(
              '${m.t('associatedProduct')}: ${data['product_id'] == 0 ? m.t('noAssociation') : '#${data['product_id']}'}',
            ),
            ExpansionTile(
              title: Text(m.t('searchProducts')),
              children: [
                NativeProductPicker(
                  selected: data['product_id'],
                  enabled: !disabled,
                  onSelected: (id) {
                    setState(() => data['product_id'] = id);
                    persist();
                  },
                ),
              ],
            ),
            space(),
            OutlinedButton(
              onPressed: disabled
                  ? null
                  : () => pick(
                      widget.kind == 'video'
                          ? 'video'
                          : data['format'] == 'pdf'
                          ? 'pdf'
                          : 'pages',
                    ),
              child: Text(
                m.t(
                  widget.kind == 'video'
                      ? 'uploadVideo'
                      : data['format'] == 'pdf'
                      ? 'uploadPDF'
                      : 'uploadPages',
                ),
              ),
            ),
            OutlinedButton(
              onPressed: disabled ? null : () => pick('cover'),
              child: Text(m.t('resourceCover')),
            ),
            for (final f in uploads)
              ListTile(
                title: Text(f['name']),
                subtitle: Text(
                  f['media_id'] != null ? m.t('ready') : m.t('queued'),
                ),
                trailing: f['media_id'] == null
                    ? IconButton(
                        onPressed: disabled
                            ? null
                            : () {
                                setState(() => uploads.remove(f));
                                persist();
                              },
                        icon: const Icon(Icons.close),
                      )
                    : null,
              ),
            if (data['format'] == 'images')
              for (var i = 0; i < (data['page_media_ids'] as List).length; i++)
                Row(
                  children: [
                    SizedBox(
                      width: 75,
                      child: Photo(id: data['page_media_ids'][i], height: 85),
                    ),
                    Text('${i + 1}'),
                    IconButton(
                      onPressed: disabled || i == 0
                          ? null
                          : () {
                              setState(() {
                                final a = data['page_media_ids'] as List;
                                final oldCover = a.first;
                                final v = a[i];
                                a[i] = a[i - 1];
                                a[i - 1] = v;
                                if (data['cover_media_id'] == oldCover) {
                                  data['cover_media_id'] = a.first;
                                }
                              });
                              persist();
                            },
                      icon: const Icon(Icons.arrow_upward),
                    ),
                    IconButton(
                      onPressed:
                          disabled ||
                              i + 1 == (data['page_media_ids'] as List).length
                          ? null
                          : () {
                              setState(() {
                                final a = data['page_media_ids'] as List;
                                final oldCover = a.first;
                                final v = a[i];
                                a[i] = a[i + 1];
                                a[i + 1] = v;
                                if (data['cover_media_id'] == oldCover) {
                                  data['cover_media_id'] = a.first;
                                }
                              });
                              persist();
                            },
                      icon: const Icon(Icons.arrow_downward),
                    ),
                    IconButton(
                      onPressed: disabled
                          ? null
                          : () {
                              setState(() {
                                final id = (data['page_media_ids'] as List)
                                    .removeAt(i);
                                if (data['cover_media_id'] == id) {
                                  data['cover_media_id'] = 0;
                                }
                                uploads.removeWhere((f) => f['media_id'] == id);
                              });
                              persist();
                            },
                      icon: const Icon(Icons.close),
                    ),
                  ],
                ),
            if (uploads.any((f) => f['media_id'] == null) && !uploading)
              FilledButton(
                onPressed: disabled ? null : upload,
                child: Text(m.t('uploadFile')),
              ),
            if (uploading) ...[
              LinearProgressIndicator(value: progress / 100),
              Text('${m.t('uploadProgress')} $progress%'),
              TextButton(
                onPressed: () {
                  cancelled = true;
                  uploadClient?.close();
                },
                child: Text(m.t('cancel')),
              ),
            ],
            if (metadata.isNotEmpty)
              message(
                context,
                '${m.t(metadata['state'])}\n${m.t(metadata['error'] ?? '')}',
              ),
            if (metadata['state'] == 'failed')
              TextButton(
                onPressed: disabled
                    ? null
                    : () async {
                        try {
                          await m.api(
                            '/merchant/resource-media/${metadata['media_id']}/retry',
                            method: 'POST',
                            data: {},
                          );
                          poll();
                        } catch (e) {
                          if (context.mounted) showError(context, e);
                        }
                      },
                child: Text(m.t('retryProcessing')),
              ),
            message(context, m.t('resourceReview')),
            OutlinedButton(
              onPressed:
                  (data['file_media_id'] != 0 ||
                      (data['page_media_ids'] as List).isNotEmpty)
                  ? () => push(
                      context,
                      NativeResourceViewer(
                        item: {
                          'kind': widget.kind,
                          'data': data,
                          'file': metadata,
                        },
                        private: true,
                      ),
                    )
                  : null,
              child: Text(m.t('resourcePreview')),
            ),
            space(),
            OutlinedButton(
              onPressed: disabled ? null : () => save(false),
              child: Text(m.t('saveCloud')),
            ),
            space(),
            FilledButton(
              onPressed:
                  disabled ||
                      metadata.isNotEmpty && metadata['state'] != 'ready'
                  ? null
                  : () => save(true),
              child: Text(m.t('submit')),
            ),
            space(24),
          ],
        ),
      ),
    );
  }
}

class NativeProductPicker extends StatefulWidget {
  final int selected;
  final bool enabled;
  final ValueChanged<int> onSelected;
  const NativeProductPicker({
    super.key,
    required this.selected,
    required this.enabled,
    required this.onSelected,
  });
  @override
  State<NativeProductPicker> createState() => _NativeProductPickerState();
}

class _NativeProductPickerState extends State<NativeProductPicker> {
  int page = 1;
  String q = '';
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Column(
      children: [
        TextButton(
          onPressed: widget.enabled ? () => widget.onSelected(0) : null,
          child: Text(m.t('noAssociation')),
        ),
        TextField(
          decoration: InputDecoration(labelText: m.t('keyword')),
          onChanged: (v) => setState(() {
            q = v;
            page = 1;
          }),
        ),
        AsyncPane(
          identity: '${m.scope}:pick:$q:$page',
          load: () => m.api(
            '/merchant/contents?kind=product&page_size=20&page=$page&q=${Uri.encodeComponent(q)}',
          ),
          builder: (r) => Column(
            children: [
              for (final i in rows(r['items']))
                ListTile(
                  title: Text(m.content(asMap(i['data']), 'name')),
                  selected: i['id'] == widget.selected,
                  onTap: widget.enabled
                      ? () => widget.onSelected(i['id'])
                      : null,
                ),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  TextButton(
                    onPressed: page > 1 ? () => setState(() => page--) : null,
                    child: Text(m.t('previous')),
                  ),
                  TextButton(
                    onPressed: page * 20 < (r['total'] as num)
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
    );
  }
}
