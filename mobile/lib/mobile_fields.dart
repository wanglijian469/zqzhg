part of 'main.dart';

/// Labels participate in layout instead of floating across an outline border.
class NativeFieldLabel extends StatelessWidget {
  final String label;
  final Widget child;
  const NativeFieldLabel({super.key, required this.label, required this.child});
  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    mainAxisSize: MainAxisSize.min,
    children: [
      Padding(
        padding: const EdgeInsets.only(bottom: 6),
        child: Text(label, style: Theme.of(context).textTheme.labelLarge),
      ),
      Semantics(label: label, child: child),
    ],
  );
}

Future<T?> nativeFilterSheet<T>(
  BuildContext context, {
  required String title,
  required List<({T value, String label})> options,
  required T selected,
}) => showModalBottomSheet<T>(
  context: context,
  isScrollControlled: true,
  useSafeArea: true,
  showDragHandle: true,
  builder: (c) => SafeArea(
    top: false,
    child: ConstrainedBox(
      constraints: BoxConstraints(maxHeight: MediaQuery.sizeOf(c).height * .75),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
            child: Text(title, style: Theme.of(c).textTheme.titleMedium),
          ),
          Flexible(
            child: ListView.builder(
              shrinkWrap: true,
              itemCount: options.length,
              itemBuilder: (c, i) => ListTile(
                title: Text(options[i].label),
                selected: options[i].value == selected,
                trailing: options[i].value == selected
                    ? const Icon(Icons.check)
                    : null,
                onTap: () => Navigator.pop(c, options[i].value),
              ),
            ),
          ),
        ],
      ),
    ),
  ),
);
