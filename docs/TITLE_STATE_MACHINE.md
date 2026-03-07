# Стейт-машина загрузки NieR Re[in]carnation

## Обзор

Стейт-машина загрузки (Title Flow) реализована как двухуровневая иерархия конечных автоматов на базе `FiniteStateMachineTask<TState, TEvent>`. 

**Верхний уровень:** `Gameplay` — управляет глобальными состояниями игры  
**Нижний уровень:** `Title` — управляет процессом загрузки от запуска до главного экрана

---

## Архитектура

### Иерархия стейт-машин

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Gameplay State Machine                        │
│  (FiniteStateMachineTask<GameplayState, GameplayEvent>)                   │
│                                                                         │
│  States: Unknown → FirstStep → Title → MainStory → ...                  │
└────────────────────────────────────┬──────────────────────────────────────┘
                                     │
                                     ▼  OnTitleAsync() запускает Title
┌─────────────────────────────────────────────────────────────────────────┐
│                            Title State Machine                          │
│  (FiniteStateMachineTask<TitleState, TitleEvent>)                       │
│                                                                         │
│  States: FirstStep → TermOfService → FirstDownload → ... → Finish       │
└─────────────────────────────────────────────────────────────────────────┘
```

### Базовый класс

```csharp
// Namespace: Adam.Framework.Coroutine
public abstract class FiniteStateMachineTask<TState, TEvent> : ISetupable
{
    protected abstract void InternalInitialize(CancellationToken cxlToken);
    protected abstract void InternalClear();
    
    public void Setup();
    public void Clear();
    
    // Переходы
    public void AddTransition(TState state, TState nextState, TEvent updateEvent, 
                              TransactionContextAsync.OnAction<TState, TEvent> onBeginAction);
}
```

---

## Title State Machine (Подробная спецификация)

### Состояния (TitleState)

| Значение | Имя | Описание |
|----------|-----|----------|
| 0 | `Unknown` | Неопределенное состояние |
| 1 | `FirstStep` | **Начальное состояние** — инициализация сети и синхронизация |
| 2 | `InPreApplication` | Предварительная загрузка (PreTitleMovie) |
| 3 | `TitleScreen` | Титульный экран с логотипом и кнопкой "Tap to Start" |
| 4 | `ApplicationVersion` | Проверка версии приложения |
| 5 | `BanAccount` | Проверка статуса блокировки аккаунта |
| 6 | `Tutorial` | Обучающий туториал |
| 7 | `TermOfService` | **Terms of Conditions** — проверка и принятие соглашений |
| 8 | `FirstDownload` | Первая загрузка ресурсов (AssetBundles) |
| 9 | `RegistUserName` | Регистрация имени пользователя / трансфер аккаунта |
| 10 | `ResolutionSetting` | Настройка качества графики |
| 11 | `Finish` | **Завершение** — переход к главному экрану |

### События (TitleEvent)

| Значение | Имя | Назначение |
|----------|-----|------------|
| 0 | `Unknown` | Неопределенное событие |
| 1 | `Start` | Общий старт |
| 2 | `StartPreApplication` | Запуск предварительной загрузки |
| 3 | `StartFormalApplication` | Запуск основного приложения |
| 4 | `CheckApplicationVersion` | Проверка версии |
| 5 | `CheckBanAccount` | Проверка бана |
| 6 | `CheckTutorial` | Проверка необходимости туториала |
| 7 | `CheckFirstDownload` | Проверка первой загрузки |
| 8 | `CheckTermOfService` | **Проверка Terms of Service** |
| 9 | `RegisterUserName` | Регистрация имени |
| 10 | `CheckResolutionSetting` | Проверка настроек разрешения |
| 11 | `Completion` | **Завершение загрузки** |

---

## Полный флоу загрузки

### Диаграмма состояний

```
                                    ┌─────────────────┐
                                    │    STARTUP      │
                                    └────────┬────────┘
                                             │
                                             ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.FirstStep                                                       │
│  └── Метод: OnFirstStep()                                                    │
│      ├── InitializeNetworkAsync()       // Инициализация сети               │
│      └── RequestSynchronousDatabaseAsync()                                   │
│          ├── SyncMasterData()           // Загрузка мастер-данных            │
│          ├── SyncUserData()             // Загрузка данных пользователя      │
│          └── SyncPurchase()             // Синхронизация покупок             │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.StartPreApplication
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.InPreApplication                                                  │
│  └── Метод: OnPreTitle()                                                     │
│      └── PreTitleMovie.Play()           // Показ intro-видео               │
│          ├── PlayMovieAsync()                                                │
│          └── Ожидание тапа или окончания видео                              │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.StartFormalApplication
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.TitleScreen                                                       │
│  └── Метод: OnTitleScreen()                                                  │
│      ├── ActForTitle.Create*TitleAsync()                                      │
│      │   ├── CreatePreDownloadTitleAsync()                                   │
│      │   ├── CreateBundleTitleAsync()                                         │
│      │   └── CreateRandomTitleAsync()   // Случайный титульный экран        │
│      ├── TitleScreen.Initialize*Async() // Загрузка UI                      │
│      │   ├── SetupTitleLogoAsync()                                           │
│      │   └── SetupCopyrightAsync()                                            │
│      └── TitleScreen.WaitTapScreen()    // Ожидание "Tap to Start"          │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.CheckTermOfService
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.TermOfService  ⬅️ ТОЧКА ВХОДА (Terms of Conditions)              │
│  └── Метод: OnTermOfService()                                                │
│      ├── FetchTermsOfServiceVersion()   // WebRequest для получения версии  │
│      ├── LoadTextData()                 // Загрузка локализованного текста  │
│      ├── TermOfServiceDialogPresenter   // UI диалог соглашений             │
│      │   └── Показ WebView с Terms of Service                               │
│      ├── OnTermOfServiceAdditionalWorldWideAsync()                           │
│      │   └── Дополнительные ToS для Worldwide версии                          │
│      └── Ожидание подтверждения пользователя                                │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.CheckFirstDownload
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.FirstDownload                                                     │
│  └── Метод: OnFirstDownload()                                                  │
│      ├── GetFirstDownloadSizeAsync()      // Определение размера загрузки   │
│      ├── InitializeAssetBundles()         // Инициализация AssetBundle       │
│      │   └── PreInitializeAssetBundle()                                      │
│      └── CanEnableForceLocalLoading()     // Проверка возможности локалoad │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.RegisterUserName
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.RegistUserName                                                    │
│  └── Метод: OnRegistUserName()                                                 │
│      ├── CheckAndInitializeCreateNewUser()                                    │
│      ├── RequestUserRegisterAndAuthAsync()                                   │
│      │   ├── IsPreApplication()           // Проверка режима                 │
│      │   ├── IsOptionalUpdate()           // Проверка обновления             │
│      │   └── ThereIsAnUpdateOfApplication()                                  │
│      ├── Регистрация / Трансфер:                                              │
│      │   ├── RequestUserRegisterAsync()     // Новый аккаунт                │
│      │   ├── PrepareTransferPlayerAsync()   // Подготовка трансфера          │
│      │   ├── TransferPlayerAsync()          // Выполнение трансфера          │
│      │   ├── RequestTransferUserAsync()     // Трансфер по ID                │
│      │   ├── RequestFaceBookTransferUserAsync()                              │
│      │   └── RequestAppleTransferUserAsync()                                 │
│      └── IsNeedGameStartApi()             // ⭐ КЛЮЧЕВОЙ API                 │
│          └── GameStart API                                                    │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.CheckApplicationVersion
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.ApplicationVersion                                                │
│  └── Метод: OnApplicationVersion()                                             │
│      └── Проверка совместимости версии клиента и сервера                     │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.CheckBanAccount
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.BanAccount                                                        │
│  └── Метод: OnBanAccount()                                                     │
│      └── Проверка статуса блокировки аккаунта                                │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.CheckTutorial
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.Tutorial                                                            │
│  └── Метод: OnTutorial() (предполагается)                                    │
│      └── Воспроизведение обучающего туториала                                │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.CheckResolutionSetting
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.ResolutionSetting                                                   │
│  └── Метод: OnGraphicQualitySetting()                                          │
│      └── Настройка качества графики                                          │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │ TitleEvent.Completion
                                          ▼
┌────────────────────────────────────────────────────────────────────────────┐
│  TitleState.Finish                                                            │
│  └── Метод: OnFinish()                                                         │
│      └── OnComplete()                     // IsCompleted = true               │
└─────────────────────────────────────────┬────────────────────────────────────┘
                                          │
                                          ▼
                              ┌─────────────────────┐
                              │  GameplayState.     │
                              │  MainStory          │
                              │  (Главный экран)    │
                              └─────────────────────┘
```

---

## Ключевые API Endpoint'ы

### gRPC API (Protobuf)

| Метод | Request | Response | Назначение |
|-------|---------|----------|------------|
| `RegisterUser` | `RegisterUserRequest` | `RegisterUserResponse` | Создание нового аккаунта |
| `TransferUser` | `TransferUserRequest` | `TransferUserResponse` | Трансфер по ID и паролю |
| `TransferUserByFacebook` | `TransferUserByFacebookRequest` | `TransferUserByFacebookResponse` | Трансфер через Facebook |
| `TransferUserByApple` | `TransferUserByAppleRequest` | `TransferUserByAppleResponse` | Трансфер через Apple ID |
| **GameStart** | **GameStartRequest** | **GameStartResponse** | **⭐ Ключевой API старта** |

### HTTP/Web API

| Метод | URL | Назначение |
|-------|-----|------------|
| `FetchTermsOfServiceVersion()` | WebRequest | Получение версии Terms of Service |

---

## UI Компоненты

### Основные компоненты

| Класс | Назначение | Ключевые методы |
|-------|------------|-----------------|
| `PreTitleMovie` | Intro-видео перед титульным экраном | `PlayMovieAsync()`, `Play()` |
| `TitleScreen` | Титульный экран | `Initialize*TitleAsync()`, `WaitTapScreen()` |
| `ActForTitle` | Фабрика создания титульных экранов | `CreatePreDownloadTitleAsync()`, `CreateBundleTitleAsync()`, `CreateRandomTitleAsync()` |
| `TermOfServiceDialogPresenter` | Диалог Terms of Service | `OnInitialize()`, `CloseDialog()` |
| `PreTitleMenuDialogPresenter` | Меню до титульного экрана | — |
| `TitleMenuDialogPresenter` | Меню на титульном экране | — |
| `TitleFlowDownloadSettingPresenter` | Настройки загрузки | — |

### TitleScreen UI элементы

```csharp
public class TitleScreen : MonoBehaviour
{
    private DarkStill[] _stillImages;           // Фоновые изображения
    private DarkTextMeshPro _playerIdText;      // ID игрока
    private DarkTextMeshPro _applicationVersionText;  // Версия приложения
    private DarkButton _menuButton;             // Кнопка меню
    private DarkButton _twitterButton;          // Кнопка Twitter
    private DarkButton _noticeButton;           // Кнопка уведомлений
    private DarkButton _movieButton;            // Кнопка видео
    private DarkButton _titleButton;            // Кнопка "Tap to Start"
    private DarkTextMeshPro _tapText;           // Текст "Tap to Start"
    private DarkImage[] _logoImages;            // Логотипы
    private DarkTextMeshPro _copyrightText;     // Копирайт
}
```

---

## Классы и их расположение

### Основные классы стейт-машины

| Класс | Namespace | Назначение |
|-------|-----------|------------|
| `Title` | `Dark.StateMachine.Title` | Главная стейт-машина загрузки |
| `IDelegator` | `Dark.StateMachine.Title` | Интерфейс делегатора |
| `TitleStubDelegator` | `Dark.StateMachine.Title` | Реализация делегатора |
| `TitleState` | — | Enum состояний |
| `TitleEvent` | — | Enum событий |
| `FiniteStateMachineTask<TState, TEvent>` | `Adam.Framework.Coroutine` | Базовый класс |

### Поля класса Title

```csharp
public class Title : FiniteStateMachineTask<TitleState, TitleEvent>
{
    private bool _isResistSuccess;              // Флаг успешной регистрации
    private bool <IsCompleted>k__BackingField; // Завершена ли загрузка
    private IDelegator _delegator;              // Делегатор для проверок
    private ActForTitle _actForTitle;           // Контроллер UI
    private Title.LoadedAssets _loadedAssets;   // Загруженные ассеты
    
    // Статические константы
    private static readonly string kButtonStateName;
}
```

### Title.LoadedAssets (Flags)

```csharp
[Flags]
private enum Title.LoadedAssets
{
    Unknown = 0,
    AudioCommon = 1,    // Общие аудио-ресурсы
    Text = 2            // Текстовые ресурсы
}
```

---

## Точки расширения

### IDelegator интерфейс

```csharp
public interface IDelegator
{
    bool IsPreApplication();              // Предварительная версия?
    bool IsOptionalUpdate();              // Доступно обновление?
    bool ThereIsAnUpdateOfApplication();  // Требуется обновление?
}
```

### Реализация TitleStubDelegator

```csharp
public class TitleStubDelegator : IDelegator
{
    public bool IsPreApplication() => false;
    public bool IsOptionalUpdate() => false;
    public bool ThereIsAnUpdateOfApplication() => false;
}
```

---

## Важные наблюдения для серверной реализации

### 1. Порядок инициализации

Клиент выполняет строгий порядок операций:
1. **Сначала** синхронизация мастер-данных (`SyncMasterData`)
2. **Затем** синхронизация данных пользователя (`SyncUserData`)
3. **Потом** показ UI и Terms of Service
4. **Только после** — регистрация и `GameStart`

### 2. GameStart API

`IsNeedGameStartApi()` — ключевой метод, определяющий необходимость вызова `GameStart`. Это **критически важный endpoint** для начала игровой сессии.

### 3. Terms of Service

- Версия ToS получается через **HTTP WebRequest**, а не gRPC
- Есть дополнительный флоу для Worldwide версии (`OnTermOfServiceAdditionalWorldWideAsync`)

### 4. Трансфер аккаунтов

Поддерживается 3 метода трансфера:
- По ID пользователя и паролю
- Через Facebook
- Через Apple ID

### 5. AssetBundles

- Первая загрузка (`FirstDownload`) включает инициализацию AssetBundles
- Есть режим `CanEnableForceLocalLoading()` для локальной загрузки

### 6. События для аналитики

Каждый переход в стейт-машине — потенциальная точка для отслеживания воронки:
- Успешные и неуспешные регистрации
- Выбор метода трансфера
- Первый вход vs возвращающийся игрок

---

## Связанные файлы кода

| Файл | Описание |
|------|----------|
| `client/il2cpp_dump/dump.cs` | Полный дамп IL2CPP с методами RVA |
| `server/gen/proto/user_service.pb.go` | gRPC протоопределения |
| `server/internal/service/user.go` | Серверная реализация User API |

---

## См. также

- [PROGRESS.md](./PROGRESS.md) — Текущий прогресс проекта
- gRPC proto файлы в `server/proto/`
- Таблицы мастер-данных в `extracted_tables/json_fixed/`

---

*Анализ произведен на основе IL2CPP дампа клиента NieR Re[in]carnation*
