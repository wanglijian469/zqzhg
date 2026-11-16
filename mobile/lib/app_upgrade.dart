part of 'main.dart';

List<Json> uniqueContent(List<Json> items) {
  final seen = <dynamic>{};
  return items.where((e) => seen.add(e['id'])).toList();
}

List<String> dialNumbers(String text) {
  final clean = text.replaceAllMapped(
    RegExp(r'[（(]([^）)]*)[）)]'),
    (m) => RegExp(r'^\d{2,5}$').hasMatch(m[1]!.trim()) ? m[1]! : ' ',
  );
  return RegExp(r'\+?\d[\d -]{5,}\d')
      .allMatches(clean)
      .map((m) => m[0]!.replaceAll(RegExp(r'[^\d+]'), ''))
      .where(
        (s) =>
            s.replaceAll('+', '').length >= 7 &&
            s.replaceAll('+', '').length <= 15,
      )
      .toSet()
      .toList();
}

Future<void> copyNativeText(BuildContext context, String text) async {
  final m = ModelScope.of(context);
  try {
    await Clipboard.setData(ClipboardData(text: text));
    if (context.mounted) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(m.t('copied'))));
    }
  } catch (_) {
    if (context.mounted) {
      await showDialog<void>(
        context: context,
        builder: (c) => AlertDialog(
          title: Text(m.t('copyFallback')),
          content: SelectableText(text),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(c),
              child: Text(m.t('close')),
            ),
          ],
        ),
      );
    }
  }
}

Future<void> dialContact(BuildContext context, String text) async {
  final m = ModelScope.of(context);
  final numbers = dialNumbers(text);
  String? selected;
  if (numbers.length == 1) {
    selected = numbers.first;
  } else if (numbers.isNotEmpty) {
    selected = await showModalBottomSheet<String>(
      context: context,
      builder: (c) => SafeArea(
        child: ListView(
          shrinkWrap: true,
          children: [
            for (final n in numbers)
              ListTile(
                minTileHeight: 56,
                leading: const Icon(Icons.phone),
                title: Text(n),
                onTap: () => Navigator.pop(c, n),
              ),
          ],
        ),
      ),
    );
  }
  if (!context.mounted) return;
  if (selected == null) {
    if (numbers.isEmpty) {
      await showDialog<void>(
        context: context,
        builder: (c) => AlertDialog(
          title: Text(m.t('phone')),
          content: SelectableText(text),
          actions: [
            TextButton(
              onPressed: () => copyNativeText(c, text),
              child: Text(m.t('copyPhone')),
            ),
            TextButton(
              onPressed: () => Navigator.pop(c),
              child: Text(m.t('close')),
            ),
          ],
        ),
      );
    }
    return;
  }
  try {
    analyticsContact(context, 'contact_phone');
    if (!await launchUrl(Uri(scheme: 'tel', path: selected))) {
      throw ApiFailure('network_error');
    }
  } catch (e) {
    if (context.mounted) showError(context, e);
  }
}

class NativeFilterPanel extends StatelessWidget {
  final List<Widget> children;
  final int count;
  final Key? panelKey;
  const NativeFilterPanel({
    super.key,
    required this.children,
    this.count = 0,
    this.panelKey,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 8),
      child: ExpansionTile(
        key: panelKey,
        maintainState: true,
        tilePadding: const EdgeInsets.symmetric(horizontal: 14),
        childrenPadding: const EdgeInsets.fromLTRB(14, 0, 14, 14),
        leading: const Icon(Icons.tune, size: 20),
        title: Text(m.t('filters') + (count > 0 ? ' · $count' : '')),
        children: [
          for (var i = 0; i < children.length; i++) ...[
            if (i > 0) space(12),
            children[i],
          ],
        ],
      ),
    );
  }
}

class NativeCompanyHeader extends StatelessWidget {
  final Json data;
  const NativeCompanyHeader({super.key, required this.data});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final p = asMap(data['company_profile'] ?? {});
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 48,
            height: 48,
            child: (p['logo_media_id'] ?? 0) > 0
                ? Photo(id: p['logo_media_id'], aspectRatio: 1)
                : const Icon(Icons.business, color: industrialBlue),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  m.content(data, 'name'),
                  style: const TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                    height: 1.4,
                  ),
                ),
                if ((data['main_products'] ?? '').toString().isNotEmpty)
                  Text(
                    data['main_products'],
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class NativeCompanyHero extends StatelessWidget {
  final Json item;
  final bool showProducts;
  final VoidCallback onProducts, onContact;
  final String slug;
  const NativeCompanyHero({
    super.key,
    required this.item,
    required this.slug,
    required this.onProducts,
    this.showProducts = true,
    required this.onContact,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final d = asMap(item['data']), p = asMap(d['company_profile'] ?? {});
    final cover =
        (p['cover_media_id'] as num?)?.toInt() ?? imageIDs(d).firstOrNull;
    return Container(
      color: const Color(0xffeaf0f8),
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          NativeCompanyHeader(data: d),
          if ((d['region_name'] ?? '').toString().isNotEmpty)
            Text(
              d['region_name'],
              style: Theme.of(context).textTheme.bodySmall,
            ),
          space(8),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: [
              if (showProducts)
                FilledButton(
                  onPressed: onProducts,
                  child: Text(m.t('viewCatalog')),
                ),
              if ((d['phone'] ?? '').toString().trim().isNotEmpty ||
                  (d['wechat'] ?? '').toString().trim().isNotEmpty)
                OutlinedButton(
                  onPressed: onContact,
                  child: Text(m.t('showroomContact')),
                ),
              IconButton(
                tooltip: m.t('shareShowroom'),
                onPressed: () => push(context, ShowroomPromotion(slug: slug)),
                icon: const Icon(Icons.share_outlined),
              ),
              FavoriteButton(item: {...item, 'kind': 'vendor'}),
            ],
          ),
          if (cover != null && cover > 0) ...[
            space(12),
            Photo(
              id: cover,
              thumb: false,
              aspectRatio: 16 / 9,
              fit: BoxFit.cover,
            ),
          ],
        ],
      ),
    );
  }
}

class ShowroomHomeBody extends StatelessWidget {
  final Json data, result;
  final ValueChanged<int> onTab;
  const ShowroomHomeBody({
    super.key,
    required this.data,
    required this.result,
    required this.onTab,
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final products = uniqueContent(
      rows(result['home_products'] ?? result['representative_products']),
    ).take(8).toList();
    final p = asMap(data['company_profile'] ?? {});
    final intro = m.content(p, 'summary').isNotEmpty
        ? m.content(p, 'summary')
        : m.content(data, 'description');
    final gallery = companyGallery(data).take(6).toList();
    Widget title(String key, int tab) => Row(
      children: [
        Expanded(
          child: Text(m.t(key), style: Theme.of(context).textTheme.titleLarge),
        ),
        TextButton(onPressed: () => onTab(tab), child: Text(m.t('all'))),
      ],
    );
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (products.isNotEmpty || result['product_count'] == 0) ...[
          title('mainProducts', 1),
          if (products.isEmpty)
            message(context, m.t('showroomEmpty'))
          else
            ExhibitGrid(items: products, showVendor: false),
        ],
        if (intro.trim().isNotEmpty &&
            intro.trim() != data['main_products']) ...[
          space(16),
          title('companyIntro', 0),
          Text(
            intro.length > 240 ? '${intro.substring(0, 240)}…' : intro,
            style: const TextStyle(height: 1.8),
          ),
        ],
        if (gallery.isNotEmpty) ...[
          space(16),
          title('showroomGallery', 0),
          NativeCompanyGallery(items: gallery),
        ],
        for (final entry in [
          ('video', 2, 'shortVideos'),
          ('catalog', 3, 'electronicCatalogs'),
        ])
          if ((asMap(result['resource_counts'] ?? {})[entry.$1] ?? 0) > 0) ...[
            space(16),
            title(entry.$3, entry.$2),
            PortalResourceSummary(
              vendorID: result['item']['id'],
              kind: entry.$1,
            ),
          ],
        space(16),
        title('showroomContact', 4),
        Contact(data: data),
      ],
    );
  }
}

class NativeCompanyGallery extends StatelessWidget {
  final List<Json> items;
  const NativeCompanyGallery({super.key, required this.items});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    return Column(
      children: [
        for (var i = 0; i < items.length; i++)
          Card(
            clipBehavior: Clip.antiAlias,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                InkWell(
                  onTap: () => push(
                    context,
                    PhotoViewer(
                      ids: items.map((r) => r['media_id'] as int).toList(),
                      initial: i,
                    ),
                  ),
                  child: Image.network(
                    '${m.baseUrl}/api/v1/media/${items[i]['media_id']}?thumb=1',
                    width: double.infinity,
                    fit: BoxFit.contain,
                    cacheWidth: 800,
                    errorBuilder: (_, _, _) => SizedBox(
                      height: 100,
                      child: Center(child: Text(m.t('noImage'))),
                    ),
                  ),
                ),
                if (m.content(items[i], 'title').isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.all(12),
                    child: Text(m.content(items[i], 'title')),
                  ),
              ],
            ),
          ),
      ],
    );
  }
}

List<int> showroomVideoImages(Json data) => [
  (asMap(data['company_profile'] ?? {})['cover_media_id'] as num? ?? 0).toInt(),
  ...imageIDs(data),
];

class NativeVideoPoster extends StatelessWidget {
  final Json data;
  final List<int> fallbackIDs;
  const NativeVideoPoster({
    super.key,
    required this.data,
    this.fallbackIDs = const [],
  });

  @override
  Widget build(BuildContext context) {
    final ids = <int>{
      (data['cover_media_id'] as num? ?? 0).toInt(),
      (data['file_media_id'] as num? ?? 0).toInt(),
      ...fallbackIDs,
    }.where((id) => id > 0).toList();
    const placeholder = ColoredBox(
      color: industrialSurface,
      child: Center(
        child: Icon(Icons.movie_outlined, size: 44, color: industrialBlue),
      ),
    );
    return ids.isEmpty
        ? const AspectRatio(aspectRatio: 16 / 9, child: placeholder)
        : ColoredBox(
            color: industrialSurface,
            child: Photo(
              id: ids.first,
              fallbackIDs: ids.skip(1).toList(),
              aspectRatio: 16 / 9,
              radius: 0,
              errorPlaceholder: placeholder,
            ),
          );
  }
}

class NativePublicResourceCard extends StatelessWidget {
  final Json item;
  final List<int> fallbackIDs;
  const NativePublicResourceCard({
    super.key,
    required this.item,
    this.fallbackIDs = const [],
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), d = asMap(item['data']);
    final file = asMap(item['file'] ?? {}), video = item['kind'] == 'video';
    final catalogImages = (d['page_media_ids'] as List? ?? []);
    final pages = (file['page_count'] as num?)?.toInt() ?? catalogImages.length;
    final cover = (d['cover_media_id'] as num? ?? 0).toInt();
    final catalogCover = cover > 0
        ? cover
        : (catalogImages.firstOrNull as num? ?? 0).toInt();
    final description = m.content(d, 'description').trim();
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () =>
            openNativeResource(context, item, fallbackIDs: fallbackIDs),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Stack(
              alignment: Alignment.center,
              children: [
                if (video)
                  NativeVideoPoster(data: d, fallbackIDs: fallbackIDs)
                else if (catalogCover > 0)
                  Photo(id: catalogCover, aspectRatio: 16 / 9)
                else
                  const AspectRatio(
                    aspectRatio: 16 / 9,
                    child: Center(
                      child: Icon(Icons.menu_book_outlined, size: 44),
                    ),
                  ),
                if (video)
                  const CircleAvatar(
                    backgroundColor: Color(0xd915324e),
                    child: Icon(Icons.play_arrow, color: Colors.white),
                  )
                else
                  const Positioned(
                    right: 12,
                    bottom: 12,
                    child: Icon(
                      Icons.menu_book,
                      color: industrialBlue,
                      size: 36,
                    ),
                  ),
              ],
            ),
            Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    resourceTitle(m, item),
                    style: Theme.of(context).textTheme.titleMedium,
                  ),
                  space(4),
                  Text(
                    video
                        ? m.t(videoPurposes[d['purpose']] ?? 'otherVideo')
                        : pages > 0
                        ? '$pages ${m.t('pageNumber')}'
                        : m.t('readCatalog'),
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                  if (video) ...[
                    space(8),
                    Text(
                      videoPlatformLabel(
                        (d['video_source'] ?? 'LOCAL').toString(),
                        m.language == 'zh',
                      ),
                      style: Theme.of(
                        context,
                      ).textTheme.labelMedium?.copyWith(color: industrialBlue),
                    ),
                  ],
                  if (isShareLinkVideo(item)) ...[
                    space(8),
                    Text(
                      shareVideoTip(
                        (d['video_source'] ?? '').toString(),
                        m.language == 'zh',
                      ),
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                    NativeShareVideoActions(item: item),
                  ],
                  if (description.isNotEmpty)
                    Text(
                      description,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
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
