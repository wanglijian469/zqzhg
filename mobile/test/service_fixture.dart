import 'dart:convert';
import 'dart:io';
import 'package:dalunongji/main.dart';

List<Json> serviceFixtureDocuments() {
  final source =
      jsonDecode(File('../server/service-documents.json').readAsStringSync())
          as List;
  return [
    for (var i = 0; i < source.length; i++)
      {
        ...asMap(source[i]),
        'id': i + 1,
        'version': 1,
        'digest': '${asMap(source[i])['key']}-v1',
        'effective_at': '2026-10-28T00:00:00Z',
        'status': 'published',
      },
  ];
}
