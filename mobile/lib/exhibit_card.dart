part of 'main.dart';

int exhibitColumns(double width, double scale) =>
    width >= 288 && scale < 1.3 ? 2 : 1;

String compactRegion(String value) => value
    .trim()
    .split(RegExp(r'[\s·•/]+'))
    .where((p) => p.isNotEmpty)
    .map((p) => p.endsWith('自治区') ? p : p.replaceFirst(RegExp(r'[省市县区]$'), ''))
    .join('·');

String productSpecification(Json data) {
  final lines = (data['parameters'] ?? '').toString().split(RegExp(r'[\r\n]+'));
  return lines
          .map((line) => line.trim())
          .where(
            (line) => RegExp(
              r'^(型号|规格|model|specification|specs?)\s*[:：]',
              caseSensitive: false,
            ).hasMatch(line),
          )
          .firstOrNull ??
      '';
}

class ExhibitSliver extends StatelessWidget {
  final List<Json> items;
  const ExhibitSliver({super.key, required this.items});
  @override
  Widget build(BuildContext context) => SliverLayoutBuilder(
    builder: (context, constraints) {
      final columns = exhibitColumns(
        constraints.crossAxisExtent,
        MediaQuery.textScalerOf(context).scale(14) / 14,
      );
      return SliverList.builder(
        itemCount: (items.length / columns).ceil(),
        itemBuilder: (context, row) => IntrinsicHeight(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              for (var col = 0; col < columns; col++) ...[
                if (col > 0) const SizedBox(width: 12),
                Expanded(
                  child: row * columns + col < items.length
                      ? ItemTile(
                          item: items[row * columns + col],
                          compact: columns == 2,
                        )
                      : const SizedBox(),
                ),
              ],
            ],
          ),
        ),
      );
    },
  );
}

class ItemTile extends StatelessWidget {
  final Json item;
  final bool compact;
  const ItemTile({super.key, required this.item, this.compact = false});
  @override
  Widget build(BuildContext context) {
    if (item['kind'] == 'vendor') return NativeVendorCard(item: item);
    final m = ModelScope.of(context);
    final data = asMap(item['data']);
    final vendor = asMap(item['vendor'] ?? {});
    final vendorData = asMap(vendor['data'] ?? vendor);
    final vendorName = m.content(vendorData, 'name');
    final region = compactRegion(
      (vendorData['region_name'] ?? data['region_name'] ?? '').toString(),
    );
    final ids = imageIDs(data);
    final spec = productSpecification(data);
    final scale = MediaQuery.textScalerOf(context);
    final titleHeight = scale.scale(15) * 1.4 * 2;
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 6),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () =>
            push(context, DetailScreen(id: item['id'], kind: 'product')),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Stack(
              children: [
                ColoredBox(
                  color: industrialSurface,
                  child: ids.isNotEmpty
                      ? Photo(
                          id: ids.first,
                          fallbackIDs: ids.skip(1).toList(),
                          aspectRatio: 1,
                          fit: BoxFit.contain,
                          radius: 0,
                        )
                      : const AspectRatio(
                          aspectRatio: 1,
                          child: Center(
                            child: Icon(
                              Icons.precision_manufacturing_outlined,
                              color: Color(0xff8ba0ba),
                              size: 36,
                            ),
                          ),
                        ),
                ),
                Positioned(
                  top: 0,
                  right: 0,
                  child: Material(
                    color: const Color(0xddffffff),
                    borderRadius: BorderRadius.circular(24),
                    child: FavoriteButton(
                      item: {...item, 'kind': 'product'},
                      iconOnly: true,
                    ),
                  ),
                ),
              ],
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(10, 10, 10, 12),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    height: titleHeight,
                    child: Text(
                      m.content(data, 'name'),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 15,
                        height: 1.4,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                  const SizedBox(height: 4),
                  SizedBox(
                    height: scale.scale(12) * 1.5,
                    child: Text(
                      spec,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                  ),
                  const SizedBox(height: 4),
                  SizedBox(
                    height: scale.scale(12) * 1.5 * 2,
                    child: Text(
                      [
                        vendorName,
                        region,
                      ].where((s) => s.isNotEmpty).join(' · '),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
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

class NativeVendorCard extends StatelessWidget {
  final Json item;
  final List<Json> productImages;
  const NativeVendorCard({
    super.key,
    required this.item,
    this.productImages = const [],
  });
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    final data = asMap(item['data']);
    final profile = asMap(data['company_profile'] ?? {});
    final images = <int>{
      if ((profile['cover_media_id'] ?? 0) > 0)
        (profile['cover_media_id'] as num).toInt(),
      ...imageIDs(data),
      for (final p in [...rows(item['matching_products']), ...productImages])
        ...imageIDs(asMap(p['data'] ?? {})),
      if ((profile['logo_media_id'] ?? 0) > 0)
        (profile['logo_media_id'] as num).toInt(),
    }.where((id) => id > 0).toList();
    final rawSlug = (item['showroom_slug'] ?? '').toString().trim();
    final slug = rawSlug.isNotEmpty ? rawSlug : 'v-${item['id']}';
    void open([int tab = 5]) =>
        push(context, CompanyShowroomScreen(slug: slug, initialTab: tab));
    final large = MediaQuery.textScalerOf(context).scale(14) / 14 >= 1.3;
    Widget actions() => LayoutBuilder(
      builder: (context, c) {
        final vertical = large || (m.language == 'en' && c.maxWidth < 260);
        final buttons = [
          FilledButton(
            key: ValueKey('showroom-${item['id']}'),
            onPressed: open,
            child: Text(m.t('enterShowroom'), maxLines: 1),
          ),
          OutlinedButton(
            key: ValueKey('contact-vendor-${item['id']}'),
            onPressed: () => open(4),
            child: Text(m.t('contact'), maxLines: 1),
          ),
        ];
        final style = ButtonStyle(
          minimumSize: const WidgetStatePropertyAll(Size(44, 44)),
          padding: const WidgetStatePropertyAll(
            EdgeInsets.symmetric(horizontal: 10, vertical: 10),
          ),
          textStyle: WidgetStatePropertyAll(
            TextStyle(
              fontFamily: Theme.of(context).textTheme.bodyMedium?.fontFamily,
              fontSize: 13,
              fontWeight: FontWeight.w600,
            ),
          ),
        );
        return FilledButtonTheme(
          data: FilledButtonThemeData(
            style: Theme.of(context).filledButtonTheme.style?.merge(style),
          ),
          child: OutlinedButtonTheme(
            data: OutlinedButtonThemeData(
              style: Theme.of(context).outlinedButtonTheme.style?.merge(style),
            ),
            child: vertical
                ? Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      buttons[0],
                      const SizedBox(height: 8),
                      buttons[1],
                    ],
                  )
                : Row(
                    children: [
                      Expanded(child: buttons[0]),
                      const SizedBox(width: 8),
                      Expanded(child: buttons[1]),
                    ],
                  ),
          ),
        );
      },
    );
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 6),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(
                  width: 104,
                  height: 104,
                  child: InkWell(
                    onTap: open,
                    borderRadius: BorderRadius.circular(10),
                    child: ColoredBox(
                      color: industrialSurface,
                      child: images.isNotEmpty
                          ? Photo(
                              id: images.first,
                              fallbackIDs: images.skip(1).toList(),
                              aspectRatio: 1,
                              fit: BoxFit.contain,
                            )
                          : const Center(
                              child: Icon(
                                Icons.factory_outlined,
                                color: Color(0xff8ba0ba),
                                size: 38,
                              ),
                            ),
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Expanded(
                            child: InkWell(
                              onTap: open,
                              child: Text(
                                m.content(data, 'name'),
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: const TextStyle(
                                  fontSize: 16,
                                  fontWeight: FontWeight.w600,
                                  height: 1.3,
                                ),
                              ),
                            ),
                          ),
                          SizedBox(
                            width: 40,
                            height: 44,
                            child: FavoriteButton(
                              item: {...item, 'kind': 'vendor'},
                              iconOnly: true,
                            ),
                          ),
                        ],
                      ),
                      Text(
                        (data['main_products'] ?? '').toString(),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          fontSize: 13,
                          height: 1.4,
                          color: Color(0xff66768a),
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        compactRegion((data['region_name'] ?? '').toString()),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: 10),
            actions(),
          ],
        ),
      ),
    );
  }
}
