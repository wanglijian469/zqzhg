import 'dart:convert';
import 'service_fixture.dart';
import 'package:http/http.dart' as http;

http.Client legacyHallClient(http.Client client) => _LegacyHallClient(client);

class _LegacyHallClient extends http.BaseClient {
  final http.Client inner;
  _LegacyHallClient(this.inner);
  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) {
    if (request.url.path.endsWith('/public/service-documents')) {
      return Future.value(
        http.StreamedResponse(
          Stream.value(
            utf8.encode(jsonEncode({'items': serviceFixtureDocuments()})),
          ),
          200,
          headers: {'content-type': 'application/json; charset=utf-8'},
        ),
      );
    }
    if (request.url.path.endsWith('/public/halls')) {
      return Future.value(
        http.StreamedResponse(
          Stream.value(utf8.encode('{"error":{"code":"not_found"}}')),
          404,
          headers: {'content-type': 'application/json'},
        ),
      );
    }
    return inner.send(request);
  }

  @override
  void close() => inner.close();
}
