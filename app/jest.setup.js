// AsyncStorage has no native module under jest; its shipped mock keeps the
// snapshot cache testable.
jest.mock('@react-native-async-storage/async-storage', () =>
  require('@react-native-async-storage/async-storage/jest/async-storage-mock'),
);
