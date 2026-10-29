part of 'main.dart';

class NativeProductQuality extends StatelessWidget {
  final Json data;
  const NativeProductQuality({super.key, required this.data});
  @override
  Widget build(BuildContext context) {
    final fields = <String, bool>{
      '产品名称': (data['name'] ?? '').toString().trim().isNotEmpty,
      '产品分类':
          ((data['product_category_id'] ?? data['category_id'] ?? 0) as num) >
          0,
      '产品封面': imageIDs(data).isNotEmpty,
      '型号信息':
          (data['model'] ?? '').toString().trim().isNotEmpty ||
          RegExp(
            r'^(型号|model)[ \t]*[:：][ \t]*\S[^\r\n]*$',
            multiLine: true,
            caseSensitive: false,
          ).hasMatch((data['parameters'] ?? '').toString()),
      '产品介绍': (data['description'] ?? '').toString().trim().isNotEmpty,
      '技术参数': (data['parameters'] ?? '')
          .toString()
          .split(RegExp(r'\r?\n'))
          .any(
            (s) =>
                s.trim().isNotEmpty && !RegExp(r'^[^:：]+[:：]\s*$').hasMatch(s),
          ),
    };
    final score = (fields.values.where((v) => v).length / fields.length * 100)
        .round();
    final en = ModelScope.of(context).language == 'en';
    final labels = [
      'Product name',
      'Category',
      'Cover',
      'Model',
      'Description',
      'Specifications',
    ];
    return Container(
      margin: const EdgeInsets.symmetric(vertical: 12),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: const Color(0xfff1f6fc),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('${en ? 'Information completeness' : '资料完整度'} $score%'),
          space(8),
          LinearProgressIndicator(value: score / 100),
          space(8),
          Wrap(
            spacing: 12,
            runSpacing: 8,
            children: fields.entries.indexed
                .map(
                  (e) => Text(
                    '${e.$2.value ? '✓' : '○'} ${en ? labels[e.$1] : e.$2.key}',
                  ),
                )
                .toList(),
          ),
          space(8),
          Text(
            en
                ? 'Use a specific name and enter models separately. The first photo is the cover; use a matching product photo. These suggestions are optional.'
                : '名称填写具体产品，型号单独填写；首图作为封面，请选对应的实物图。提示不增加必填项，不改写原资料。',
          ),
        ],
      ),
    );
  }
}
