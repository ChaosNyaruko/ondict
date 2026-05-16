 ANDROID_HOME=~/Library/Android/sdk \
 ANDROID_NDK_HOME=~/Library/Android/sdk/ndk/30.0.14904198 \
 PATH=$PATH:$(go env GOPATH)/bin \
 gomobile bind -target=android/arm64 -androidapi 36 -o /tmp/mobile.aar ./mobile/ 2>&1
