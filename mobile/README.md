# tabitha mobile wrapper

The thinnest possible native shell: a [Capacitor](https://capacitorjs.com)
config that points an iOS/Android WebView straight at the live site
(`server.url` in `capacitor.config.json`) — no bundled web app, no JS
framework, nothing to build. tabitha itself doesn't change; this just gets
it an app icon and a spot on the home screen.

Because everything actually renders from the live site, this app also gets
the PWA offline support already built into tabitha (`static/sw.js`,
`static/js/offline-sync.js`) for free — see the caveat about iOS below.

`ios/` and `android/` aren't committed here: they're full native projects
`cap add` generates from `capacitor.config.json`, and generating/building
them needs Xcode/Android Studio, which this repo's remote dev environment
doesn't have. Generate them locally instead.

## Before you start

Update `capacitor.config.json`'s `server.url` if `https://tabitha.jakehash.com`
isn't actually where tabitha is deployed, and change `appId` if
`com.jakehash.tabitha` isn't the reverse-DNS ID you want in the App Store /
Play Store.

## iOS (on your MacBook)

Requires [Xcode](https://apps.apple.com/us/app/xcode/id497799835) and
[CocoaPods](https://cocoapods.org) (`sudo gem install cocoapods`), plus
Node.js.

```sh
cd mobile
npm install
npx cap add ios       # generates ios/ — a real Xcode project
npx cap open ios       # opens it in Xcode
```

From Xcode: pick a simulator or your plugged-in iPhone, hit Run. That's the
whole build — there's no separate "build the web app" step since it's
never bundled.

After changing `capacitor.config.json` (e.g. a different `server.url`),
re-run `npx cap sync ios` to push the change into the generated project.

**iOS offline caveat:** Service workers only work in `WKWebView` (what
Capacitor uses) starting **iOS 17**. On iOS 16 and earlier, `static/sw.js`
never registers, so the offline mode this app otherwise gets for free
silently doesn't apply — the app just needs a live connection, same as
any other web view. If that matters, set the Xcode project's iOS
Deployment Target to 17+ under the App target's General tab.

## Android

Requires [Android Studio](https://developer.android.com/studio), plus
Node.js. Android's WebView is Chromium-based and has supported service
workers for years, so offline mode works there without the iOS caveat
above, on any reasonably current device.

```sh
cd mobile
npm install                # if not already done for iOS
npx cap add android         # generates android/ — a real Android Studio project
npx cap open android         # opens it in Android Studio
```

Pick a device/emulator, hit Run. Same as iOS: no separate web build step.

After changing `capacitor.config.json`, re-run `npx cap sync android`.

## What's not set up

App icon / splash screen ([`@capacitor/assets`](https://github.com/ionic-team/capacitor-assets)
can generate both from `static/icons/icon-512.png` once you want a real
app icon instead of Capacitor's placeholder — not done here to keep this
scaffold minimal), and app store signing/provisioning, which is
account-specific and has to happen in Xcode/Play Console directly.
