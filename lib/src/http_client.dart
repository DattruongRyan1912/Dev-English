import 'package:http/http.dart' as http;

import 'http_client_io.dart'
    if (dart.library.js_interop) 'http_client_web.dart'
    as platform;

http.Client createHttpClient() => platform.createHttpClient();
