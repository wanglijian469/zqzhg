part of 'main.dart';

const workspaceChannel = MethodChannel('com.zqzt.dalunongji/workspace');

/// Public map reuses the bundled website and the app's fixed API origin.
/// It never transfers the signed-in account or session to the WebView.
Future<void> openManufacturerMap(BuildContext context) async {
  final m = ModelScope.of(context);
  try {
    if (Platform.isAndroid) {
      final config = asMap(await m.api('/public/config'));
      if (!context.mounted) return;
      await workspaceChannel.invokeMethod('open', {
        'baseUrl': m.baseUrl,
        'path': '/regions?hall=${Uri.encodeComponent(m.activeHall.slug)}',
        'title': syncText(context, '厂商地图', 'Manufacturer map'),
        'language': m.language,
        'config': config,
        'token': '',
        'account': <String, dynamic>{},
      });
    } else {
      final opened = await launchUrl(
        Uri.parse(
          '${m.baseUrl}/regions?hall=${Uri.encodeComponent(m.activeHall.slug)}',
        ),
        mode: LaunchMode.externalApplication,
      );
      if (!opened) throw ApiFailure('network_error');
    }
  } catch (e) {
    if (context.mounted) showError(context, e);
  }
}

bool validWorkspacePath(String path) {
  final uri = Uri.tryParse(path);
  if (uri == null ||
      uri.hasScheme ||
      uri.hasAuthority ||
      !path.startsWith('/')) {
    return false;
  }
  if (uri.pathSegments.any((s) => s == '..' || s == '.')) return false;
  return !path.contains(RegExp(r'[\\\x00-\x20]')) &&
      (RegExp(r'^/procurement(?:/[1-9][0-9]*)?$').hasMatch(uri.path) ||
          RegExp(r'^/buyer(?:/|$)').hasMatch(uri.path) ||
          RegExp(r'^/merchant/opportunities(?:/|$)').hasMatch(uri.path));
}

Future<void> openProcurementWorkspace(
  BuildContext context, {
  String path = '/buyer',
  bool supplier = false,
}) async {
  await showDialog<void>(
    context: context,
    builder: (c) => AlertDialog(
      title: Text(syncText(c, '功能暂时暂停', 'Service temporarily paused')),
      content: Text(
        syncText(
          c,
          '请通过厂商官网联系厂商。',
          'Contact the manufacturer through their website.',
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(c),
          child: Text(syncText(c, '关闭', 'Close')),
        ),
      ],
    ),
  );
  return;
}

class ProcurementEntry extends StatefulWidget {
  final bool supplier;
  final int? productID;
  const ProcurementEntry({super.key, this.supplier = false, this.productID});
  @override
  State<ProcurementEntry> createState() => _ProcurementEntryState();
}

class _ProcurementEntryState extends State<ProcurementEntry> {
  @override
  Widget build(BuildContext context) => const SizedBox.shrink();
}
