// All user-facing strings, tech.md section 15.
export const t = {
  appName: 'Кальянный таймер',
  status: {
    free: 'Свободен',
    green: 'Работает',
    yellow: 'Нужно внимание',
    red: 'Срочно',
  },
  ui: {
    close: 'Закрыть',
    cancel: 'Отмена',
    decrease: 'Меньше',
    increase: 'Больше',
    code: 'Код заведения',
    retry: 'Повторить',
  },
  tabs: {
    hall: 'Зал',
    tasks: 'Задачи',
  },
  hall: {
    loading: 'Загружаем зал',
    noHalls: 'Залов пока нет',
    noHallsHint: 'Администратор создаёт залы в настройках',
    online: 'На связи',
    offline: 'Нет связи',
    connecting: 'Подключаемся',
    clockOffset: (sec: number) => `Сдвиг часов: ${sec} с`,
    seq: (seq: number) => `Событие №${seq}`,
  },
  me: {
    role: { owner: 'Владелец', admin: 'Администратор', master: 'Мастер' },
    joinCode: (code: string) => `Код входа: ${code}`,
    loadFailed: 'Не удалось загрузить профиль',
  },
  join: {
    title: 'Вход в смену',
    soon: 'Вход по коду появится в следующем обновлении',
  },
  tasks: {
    empty: 'Задач нет',
    emptyHint: 'Задачи появятся, когда столам понадобится сервис',
  },
  table: {
    title: (label: string) => `Стол ${label}`,
  },
  errors: {
    network: 'Нет связи с сервером',
    unknown: 'Что-то пошло не так',
  },
  settings: {
    title: 'Настройки',
  },
  kitchen: {
    title: 'Примитивы',
  },
} as const;
