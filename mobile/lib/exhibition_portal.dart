part of 'main.dart';

class ExhibitionPortalScreen extends StatelessWidget {
  final ValueChanged<ExhibitionHall> onSelect;
  const ExhibitionPortalScreen({super.key, required this.onSelect});
  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context);
    if (!m.hallsReady) {
      unawaited(m.loadHalls());
      return const HallLoadScreen();
    }
    return Scaffold(
      appBar: appBar(context, m.t('brand')),
      body: RefreshIndicator(
        onRefresh: () => m.loadHalls(refresh: true),
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Text(
              syncText(context, '行业展厅', 'Industry halls'),
              style: Theme.of(context).textTheme.headlineSmall,
            ),
            space(8),
            Text(
              syncText(
                context,
                '选择行业展厅，走进厂商展台，了解企业与产品。',
                'Choose a hall to discover manufacturers and products.',
              ),
            ),
            space(16),
            for (final hall in m.availableHalls)
              Card(
                child: InkWell(
                  onTap: () => onSelect(hall),
                  borderRadius: BorderRadius.circular(12),
                  child: Padding(
                    padding: const EdgeInsets.all(20),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Icon(
                          hall.slug == 'agriculture'
                              ? Icons.agriculture_outlined
                              : Icons.factory_outlined,
                          size: 40,
                          color: industrialBlue,
                        ),
                        space(12),
                        Text(
                          exhibitionHallName(m, hall),
                          style: Theme.of(context).textTheme.titleLarge,
                        ),
                        space(8),
                        Text(
                          hall.isOpen
                              ? syncText(context, '开放中', 'Open')
                              : m.t('hallPreparing'),
                        ),
                        space(12),
                        Text(
                          (m.language == 'en'
                                      ? hall.descriptionEN
                                      : hall.description)
                                  .isEmpty
                              ? syncText(
                                  context,
                                  '查看厂商与产品',
                                  'Explore manufacturers and products',
                                )
                              : (m.language == 'en'
                                    ? hall.descriptionEN
                                    : hall.description),
                        ),
                        space(12),
                        Text(syncText(context, '进入展厅 →', 'Enter hall →')),
                      ],
                    ),
                  ),
                ),
              ),
            space(20),
            const HomeJoinCard(),
            space(16),
            const ICPRegistrationTile(),
          ],
        ),
      ),
    );
  }
}

class ICPRegistrationTile extends StatelessWidget {
  final String number;
  const ICPRegistrationTile({super.key, this.number = '冀ICP备2026047360号-2'});
  @override
  Widget build(BuildContext context) => TextButton(
    onPressed: () => launchUrl(
      Uri.parse('https://beian.miit.gov.cn/'),
      mode: LaunchMode.externalApplication,
    ),
    child: Text(
      number,
      textAlign: TextAlign.center,
      style: const TextStyle(fontSize: 12),
    ),
  );
}

class AppAboutCard extends StatefulWidget {
  const AppAboutCard({super.key});
  @override
  State<AppAboutCard> createState() => _AppAboutCardState();
}

class _AppAboutCardState extends State<AppAboutCard> {
  Future<dynamic>? config;
  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    config ??= ModelScope.of(context).api('/public/config');
  }

  @override
  Widget build(BuildContext context) => Card(
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        children: [
          const Text(
            '大陆农机',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
          ),
          const Text('中企展馆-农业机械厅'),
          const Text('1.10.4 (42)'),
          FutureBuilder<dynamic>(
            future: config,
            builder: (context, snapshot) {
              final site = asMap(asMap(snapshot.data ?? {})['site'] ?? {});
              return Column(
                children: [
                  ICPRegistrationTile(
                    number: (site['icp_number'] ?? '冀ICP备2026047360号-2')
                        .toString(),
                  ),
                  if ((site['app_number'] ?? '').toString().isNotEmpty)
                    ICPRegistrationTile(number: site['app_number'].toString()),
                  if ((site['police_number'] ?? '').toString().isNotEmpty)
                    TextButton(
                      onPressed: () => launchUrl(
                        Uri.parse(
                          (site['police_url'] ?? '').toString().isEmpty
                              ? 'https://beian.mps.gov.cn/'
                              : site['police_url'].toString(),
                        ),
                        mode: LaunchMode.externalApplication,
                      ),
                      child: Text(site['police_number'].toString()),
                    ),
                ],
              );
            },
          ),
        ],
      ),
    ),
  );
}
