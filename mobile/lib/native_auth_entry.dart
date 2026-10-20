part of 'main.dart';

class LoginScreen extends StatefulWidget {
  final bool visitor, register;
  final String initialName;
  final VoidCallback? onRegistered;
  const LoginScreen({
    super.key,
    this.visitor = false,
    this.register = false,
    this.initialName = '',
    this.onRegistered,
  });
  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final username = TextEditingController(),
      password = TextEditingController(),
      confirmation = TextEditingController(),
      phone = TextEditingController(),
      code = TextEditingController();
  final names = FocusNode(),
      passwordFocus = FocusNode(),
      confirmFocus = FocusNode(),
      phoneFocus = FocusNode(),
      codeFocus = FocusNode();
  bool registerMode = false,
      accountMode = true,
      manufacturerLogin = false,
      busy = false,
      visible = false,
      confirmationVisible = false,
      unconfirmed = false;
  bool get visitorAccount => widget.visitor && !manufacturerLogin;
  Map<String, String> errors = {};
  String error = '', dev = '';
  int cooldown = 0;
  Timer? timer;
  @override
  void initState() {
    super.initState();
    registerMode = widget.register;
    username.text = widget.initialName;
  }

  @override
  void dispose() {
    for (final c in [username, password, confirmation, phone, code]) {
      c.dispose();
    }
    for (final n in [
      names,
      passwordFocus,
      confirmFocus,
      phoneFocus,
      codeFocus,
    ]) {
      n.dispose();
    }
    timer?.cancel();
    super.dispose();
  }

  void mode({bool? register, bool? account, bool? manufacturer}) {
    if (busy) return;
    setState(() {
      registerMode = register ?? registerMode;
      accountMode = account ?? accountMode;
      manufacturerLogin = manufacturer ?? manufacturerLogin;
      password.clear();
      confirmation.clear();
      visible = confirmationVisible = false;
      errors = {};
      error = '';
      unconfirmed = false;
    });
  }

  void focusErrors() {
    final focus = errors.containsKey('login_name')
        ? names
        : errors.containsKey('password')
        ? passwordFocus
        : errors.containsKey('confirmation')
        ? confirmFocus
        : errors.containsKey('phone')
        ? phoneFocus
        : codeFocus;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) focus.requestFocus();
    });
  }

  Future<void> submit({bool send = false}) async {
    if (busy || send && cooldown > 0) return;
    final m = ModelScope.of(context), source = m.baseUrl;
    final invalid = <String, String>{};
    if (accountMode) {
      if (username.text.trim().isEmpty ||
          username.text.trim().runes.length > 100) {
        invalid['login_name'] = syncText(
          context,
          '账号不能为空，最多100字',
          'Required, up to 100 characters',
        );
      }
      if (registerMode) {
        final n = utf8.encode(password.text).length;
        if (n < 12 || n > 72 || password.text.trim().isEmpty) {
          invalid['password'] = syncText(
            context,
            '密码需12–72个UTF-8字节',
            'Use 12–72 UTF-8 bytes',
          );
        }
        if (password.text != confirmation.text) {
          invalid['confirmation'] = syncText(
            context,
            '两次密码不一致',
            'Passwords do not match',
          );
        }
      } else if (password.text.isEmpty) {
        invalid['password'] = syncText(context, '请填写密码', 'Enter your password');
      }
    } else {
      if (phone.text.trim().isEmpty) {
        invalid['phone'] = syncText(
          context,
          '请填写手机号',
          'Enter your mobile number',
        );
      }
      if (!send && code.text.trim().isEmpty) {
        invalid['code'] = syncText(context, '请填写验证码', 'Enter the code');
      }
    }
    setState(() {
      errors = invalid;
      error = '';
    });
    if (invalid.isNotEmpty) {
      focusErrors();
      return;
    }
    setState(() => busy = true);
    try {
      final path = accountMode
          ? (visitorAccount
                ? (registerMode ? '/auth/visitor/register' : '/auth/visitor')
                : (registerMode ? '/auth/merchant/register' : '/auth/merchant'))
          : (send ? '/auth/sms' : '/auth/phone');
      final payload = accountMode
          ? {
              (registerMode ? 'login_name' : 'username'): username.text.trim(),
              'password': password.text,
            }
          : {'phone': phone.text.trim(), if (!send) 'code': code.text.trim()};
      final response = asMap(await m.api(path, method: 'POST', data: payload));
      if (!mounted || m.baseUrl != source) return;
      if (send) {
        setState(() {
          dev = response['development_code'] ?? '';
          cooldown = 60;
        });
        timer?.cancel();
        timer = Timer.periodic(const Duration(seconds: 1), (t) {
          if (!mounted) {
            t.cancel();
            return;
          }
          setState(() => cooldown--);
          if (cooldown <= 0) t.cancel();
        });
      } else {
        await m.signIn(response);
        if (!mounted) return;
        password.clear();
        confirmation.clear();
        if (registerMode && !visitorAccount) widget.onRegistered?.call();
        if (mounted) Navigator.pop(context, true);
      }
    } catch (e) {
      if (!mounted || m.baseUrl != source) return;
      final failure = e is ApiFailure ? e : ApiFailure('network_error');
      setState(() {
        if (failure.fields.isNotEmpty) {
          errors = {
            for (final f in failure.fields) f['field']: fieldIssueText(m, f),
          };
        } else if (['invalid_phone', 'invalid_code'].contains(failure.code)) {
          errors = {
            failure.code == 'invalid_phone' ? 'phone' : 'code': m.t(
              failure.code,
            ),
          };
        } else {
          error = m.t(failure.code);
        }
        if (registerMode && failure.code == 'network_error') {
          unconfirmed = true;
          error = syncText(
            context,
            '无法确认账号是否已注册。请先用刚设置的账号密码登录继续，不会自动重复注册。',
            'Registration could not be confirmed. Sign in with these credentials; registration is not retried automatically.',
          );
        }
      });
      if (errors.isNotEmpty) focusErrors();
    } finally {
      if (mounted) setState(() => busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final m = ModelScope.of(context), zh = m.language == 'zh';
    Widget input(
      TextEditingController controller,
      FocusNode focus,
      String label,
      String name, {
      bool secret = false,
      bool confirm = false,
      TextInputType? keyboard,
    }) => Padding(
      padding: const EdgeInsets.only(bottom: 16),
      child: TextField(
        controller: controller,
        focusNode: focus,
        enabled: !busy,
        obscureText: secret && !(confirm ? confirmationVisible : visible),
        keyboardType: keyboard,
        autocorrect: !secret,
        enableSuggestions: !secret,
        textInputAction: TextInputAction.done,
        onSubmitted: (_) => submit(),
        autofillHints: name == 'login_name'
            ? [AutofillHints.username]
            : secret
            ? [
                registerMode
                    ? AutofillHints.newPassword
                    : AutofillHints.password,
              ]
            : name == 'phone'
            ? [AutofillHints.telephoneNumberNational]
            : name == 'code'
            ? [AutofillHints.oneTimeCode]
            : null,
        onChanged: (_) => setState(() {
          errors.remove(name);
          if (name == 'phone') {
            code.clear();
            dev = '';
          }
        }),
        decoration: InputDecoration(
          labelText: label,
          errorText: errors[name],
          errorMaxLines: 3,
          suffixIcon: secret
              ? IconButton(
                  tooltip: (confirm ? confirmationVisible : visible)
                      ? (zh ? '隐藏$label' : 'Hide $label')
                      : (zh ? '显示$label' : 'Show $label'),
                  onPressed: busy
                      ? null
                      : () => setState(() {
                          if (confirm) {
                            confirmationVisible = !confirmationVisible;
                          } else {
                            visible = !visible;
                          }
                        }),
                  icon: Icon(
                    (confirm ? confirmationVisible : visible)
                        ? Icons.visibility_off_outlined
                        : Icons.visibility_outlined,
                  ),
                )
              : null,
        ),
      ),
    );
    final card = Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: AutofillGroup(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                registerMode
                    ? (visitorAccount
                          ? (zh ? '注册访客账号' : 'Create a visitor account')
                          : (zh ? '注册公司账号' : 'Create a company account'))
                    : (visitorAccount
                          ? (zh ? '访客账号登录' : 'Visitor sign-in')
                          : (zh ? '登录厂商工作台' : 'Sign in to your workspace')),
                style: const TextStyle(
                  fontSize: 23,
                  fontWeight: FontWeight.bold,
                ),
              ),
              space(8),
              Text(
                registerMode
                    ? (zh
                          ? '设置账号密码，下一步完善企业资料。'
                          : 'Set account details, then complete your profile.')
                    : (zh
                          ? '使用公司账号，继续管理企业。'
                          : 'Use your company account to continue.'),
              ),
              space(20),
              if (widget.visitor) ...[
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    for (final value in [false, true])
                      ChoiceChip(
                        label: Text(
                          m.t(value ? 'manufacturerAccount' : 'visitorAccount'),
                        ),
                        selected: manufacturerLogin == value,
                        onSelected: busy
                            ? null
                            : (_) => mode(manufacturer: value),
                      ),
                  ],
                ),
                space(16),
              ],
              if (!registerMode) ...[
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    for (final value in [true, false])
                      ChoiceChip(
                        label: Text(m.t(value ? 'accountLogin' : 'smsLogin')),
                        selected: accountMode == value,
                        onSelected: busy ? null : (_) => mode(account: value),
                      ),
                  ],
                ),
                space(20),
              ],
              if (accountMode) ...[
                input(
                  username,
                  names,
                  visitorAccount
                      ? m.t('username')
                      : (zh ? '公司登录账号' : 'Company login name'),
                  'login_name',
                ),
                input(
                  password,
                  passwordFocus,
                  registerMode && !visitorAccount
                      ? (zh ? '设置密码' : 'Set password')
                      : m.t('password'),
                  'password',
                  secret: true,
                ),
                if (registerMode) ...[
                  input(
                    confirmation,
                    confirmFocus,
                    zh ? '确认密码' : 'Confirm password',
                    'confirmation',
                    secret: true,
                    confirm: true,
                  ),
                  Text(
                    zh
                        ? '账号最多100字；密码12–72字节。'
                        : 'Login name up to 100 characters; password 12–72 bytes.',
                    style: const TextStyle(
                      fontSize: 12,
                      color: Color(0xff637b95),
                    ),
                  ),
                  ExpansionTile(
                    tilePadding: EdgeInsets.zero,
                    title: Text(
                      zh ? '密码要求说明' : 'Password requirements',
                      style: const TextStyle(fontSize: 13),
                    ),
                    children: [
                      Text(
                        zh
                            ? '不能全为空白。中文通常占3字节、emoji通常占4字节；空格保留。'
                            : 'Not all whitespace. Chinese characters usually use 3 bytes, emoji 4; spaces are preserved.',
                      ),
                    ],
                  ),
                ],
              ],
              if (!accountMode) ...[
                Text(
                  zh
                      ? '短信登录管理手机号对应账号，不会自动接管公司账号展厅。'
                      : 'SMS sign-in uses the phone account and does not claim a company showroom.',
                ),
                space(),
                input(
                  phone,
                  phoneFocus,
                  zh ? '手机号' : 'Mobile number',
                  'phone',
                  keyboard: TextInputType.phone,
                ),
                input(
                  code,
                  codeFocus,
                  m.t('code'),
                  'code',
                  keyboard: TextInputType.number,
                ),
                OutlinedButton(
                  onPressed: busy || cooldown > 0
                      ? null
                      : () => submit(send: true),
                  child: Text(cooldown > 0 ? '$cooldown s' : m.t('sendCode')),
                ),
                if (dev.isNotEmpty) message(context, '${m.t('devCode')}: $dev'),
              ],
              if (error.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 12),
                  child: Text(error, style: const TextStyle(color: Colors.red)),
                ),
              FilledButton(
                onPressed: busy ? null : () => submit(),
                child: Text(
                  busy
                      ? m.t('loading')
                      : registerMode
                      ? (zh ? '注册并继续' : 'Register and continue')
                      : accountMode
                      ? m.t('login')
                      : m.t('verify'),
                ),
              ),
              if (unconfirmed)
                TextButton(
                  onPressed: busy
                      ? null
                      : () => setState(() {
                          registerMode = false;
                          accountMode = true;
                          visible = confirmationVisible = false;
                          confirmation.clear();
                          password.clear();
                          error = '';
                          unconfirmed = false;
                        }),
                  child: Text(
                    zh
                        ? '已有账号？登录继续'
                        : 'Already registered? Sign in to continue',
                  ),
                ),
              const Divider(height: 32),
              TextButton(
                onPressed: busy
                    ? null
                    : () => mode(register: !registerMode, account: true),
                child: Text(
                  registerMode
                      ? (visitorAccount
                            ? (zh ? '已有访客账号登录' : 'Visitor sign-in')
                            : (zh ? '已有公司账号登录' : 'Company sign-in'))
                      : (visitorAccount
                            ? m.t('newVisitorRegister')
                            : (zh ? '新厂商注册' : 'Register manufacturer')),
                ),
              ),
              if (!registerMode)
                ExpansionTile(
                  tilePadding: EdgeInsets.zero,
                  title: Text(
                    zh ? '登录帮助' : 'Sign-in help',
                    style: const TextStyle(fontSize: 13),
                  ),
                  children: [
                    Text(
                      zh
                          ? '忘记公司密码时，请联系平台管理员通过现有后台重置。已有展厅使用原公司账号，避免重复注册。'
                          : 'Ask your platform administrator to reset a forgotten password. Use the original company account for an existing showroom.',
                    ),
                  ],
                ),
            ],
          ),
        ),
      ),
    );
    final intro = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          registerMode
              ? (zh ? '厂商入驻' : 'Join as a manufacturer')
              : (zh ? '厂商登录' : 'Manufacturer sign-in'),
          style: const TextStyle(fontSize: 28, fontWeight: FontWeight.bold),
        ),
        space(10),
        Text(
          registerMode
              ? (zh
                    ? '创建公司账号 → 完善企业资料 → 提交审核'
                    : 'Create account → Complete profile → Submit for review')
              : (zh ? '企业展厅 · 产品管理 · 展厅推广' : 'Showroom · Products · Promotion'),
        ),
        space(16),
      ],
    );
    return Theme(
      data: merchantTheme(context),
      child: Scaffold(
        appBar: appBar(
          context,
          widget.visitor
              ? m.t('favoriteAccountLogin')
              : (registerMode
                    ? (zh ? '我要入驻' : 'Exhibit with us')
                    : (zh ? '厂商登录' : 'Manufacturer sign-in')),
        ),
        body: SafeArea(
          child: LayoutBuilder(
            builder: (context, c) => SingleChildScrollView(
              padding: const EdgeInsets.all(20),
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 1040),
                  child: c.maxWidth >= 960
                      ? Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Expanded(child: intro),
                            const SizedBox(width: 40),
                            SizedBox(width: 420, child: card),
                          ],
                        )
                      : ConstrainedBox(
                          constraints: const BoxConstraints(maxWidth: 480),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.stretch,
                            children: [intro, card],
                          ),
                        ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

Future<void> authenticateManufacturerEntry(
  BuildContext context, {
  bool register = true,
}) async {
  final m = ModelScope.of(context), source = m.baseUrl;
  bool registered = false;
  final ok = await Navigator.push<bool>(
    context,
    MaterialPageRoute(
      builder: (_) => LoginScreen(
        register: register,
        onRegistered: () => registered = true,
      ),
    ),
  );
  if (!context.mounted || ok != true || m.baseUrl != source || !isMerchant(m)) {
    return;
  }
  if (registered) {
    m.newlyRegistered = true;
    await editMerchant(context, null, 'vendor');
  } else {
    push(context, const MerchantScreen());
  }
}
