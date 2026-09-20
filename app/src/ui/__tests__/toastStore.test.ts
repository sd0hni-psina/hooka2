import { useToastStore } from '../toastStore';

describe('toast queue', () => {
  beforeEach(() => {
    jest.useFakeTimers();
    useToastStore.getState().clear();
  });
  afterEach(() => {
    jest.useRealTimers();
  });

  it('shows what it was given', () => {
    useToastStore.getState().show({ message: 'Нет связи', tone: 'danger' });
    expect(useToastStore.getState().items.map((i) => i.message)).toEqual(['Нет связи']);
  });

  it('collapses repeats of the same key', () => {
    for (let i = 0; i < 5; i += 1) {
      useToastStore.getState().show({ key: 'net', message: `попытка ${i}`, tone: 'danger' });
    }
    expect(useToastStore.getState().items).toHaveLength(1);
    expect(useToastStore.getState().items[0]?.message).toBe('попытка 4');
  });

  it('keeps at most three messages on screen', () => {
    for (let i = 0; i < 6; i += 1) {
      useToastStore.getState().show({ message: `стол ${i}`, tone: 'warning' });
    }
    expect(useToastStore.getState().items.map((i) => i.message)).toEqual(['стол 3', 'стол 4', 'стол 5']);
  });

  it('takes a message down when its time runs out', () => {
    useToastStore.getState().show({ message: 'Сохранено', tone: 'success', ttlMs: 1000 });
    jest.advanceTimersByTime(999);
    expect(useToastStore.getState().items).toHaveLength(1);
    jest.advanceTimersByTime(2);
    expect(useToastStore.getState().items).toHaveLength(0);
  });

  it('restarts the clock when the same key speaks again', () => {
    useToastStore.getState().show({ key: 'net', message: 'раз', tone: 'danger', ttlMs: 1000 });
    jest.advanceTimersByTime(800);
    useToastStore.getState().show({ key: 'net', message: 'два', tone: 'danger', ttlMs: 1000 });
    jest.advanceTimersByTime(800);
    expect(useToastStore.getState().items).toHaveLength(1);
    jest.advanceTimersByTime(300);
    expect(useToastStore.getState().items).toHaveLength(0);
  });
});
