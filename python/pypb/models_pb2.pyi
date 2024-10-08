from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Iterable as _Iterable, Mapping as _Mapping, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ThreadingType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    Sequential: _ClassVar[ThreadingType]
    Parallel: _ClassVar[ThreadingType]

class GatewayType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    exclusive: _ClassVar[GatewayType]
    inclusive: _ClassVar[GatewayType]
    parallel: _ClassVar[GatewayType]

class GatewayDirection(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    divergent: _ClassVar[GatewayDirection]
    convergent: _ClassVar[GatewayDirection]

class ProcessHistoryType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    processExecute: _ClassVar[ProcessHistoryType]
    activityExecute: _ClassVar[ProcessHistoryType]
    activityComplete: _ClassVar[ProcessHistoryType]
    processSpawnSync: _ClassVar[ProcessHistoryType]
    processComplete: _ClassVar[ProcessHistoryType]
    processAbort: _ClassVar[ProcessHistoryType]
    activityAbort: _ClassVar[ProcessHistoryType]
    jobAbort: _ClassVar[ProcessHistoryType]
    jobExecute: _ClassVar[ProcessHistoryType]
    jobComplete: _ClassVar[ProcessHistoryType]
    compensationCheckpoint: _ClassVar[ProcessHistoryType]

class WorkflowTimerType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    duration: _ClassVar[WorkflowTimerType]
    fixed: _ClassVar[WorkflowTimerType]

class RecipientType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    job: _ClassVar[RecipientType]

class CancellationState(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    executing: _ClassVar[CancellationState]
    completed: _ClassVar[CancellationState]
    terminated: _ClassVar[CancellationState]
    errored: _ClassVar[CancellationState]
    obsolete: _ClassVar[CancellationState]
    compensating: _ClassVar[CancellationState]

class LogSource(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    logSourceEngine: _ClassVar[LogSource]
    logSourceWorkflow: _ClassVar[LogSource]
    logSourceClient: _ClassVar[LogSource]
    logSourceJob: _ClassVar[LogSource]
    logSourceTelemetry: _ClassVar[LogSource]

class RetryStrategy(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    Linear: _ClassVar[RetryStrategy]
    Incremental: _ClassVar[RetryStrategy]

class RetryErrorAction(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    PauseWorkflow: _ClassVar[RetryErrorAction]
    ThrowWorkflowError: _ClassVar[RetryErrorAction]
    SetVariableValue: _ClassVar[RetryErrorAction]
    FailWorkflow: _ClassVar[RetryErrorAction]

class HandlingStrategy(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    Pause: _ClassVar[HandlingStrategy]
    TearDown: _ClassVar[HandlingStrategy]
Sequential: ThreadingType
Parallel: ThreadingType
exclusive: GatewayType
inclusive: GatewayType
parallel: GatewayType
divergent: GatewayDirection
convergent: GatewayDirection
processExecute: ProcessHistoryType
activityExecute: ProcessHistoryType
activityComplete: ProcessHistoryType
processSpawnSync: ProcessHistoryType
processComplete: ProcessHistoryType
processAbort: ProcessHistoryType
activityAbort: ProcessHistoryType
jobAbort: ProcessHistoryType
jobExecute: ProcessHistoryType
jobComplete: ProcessHistoryType
compensationCheckpoint: ProcessHistoryType
duration: WorkflowTimerType
fixed: WorkflowTimerType
job: RecipientType
executing: CancellationState
completed: CancellationState
terminated: CancellationState
errored: CancellationState
obsolete: CancellationState
compensating: CancellationState
logSourceEngine: LogSource
logSourceWorkflow: LogSource
logSourceClient: LogSource
logSourceJob: LogSource
logSourceTelemetry: LogSource
Linear: RetryStrategy
Incremental: RetryStrategy
PauseWorkflow: RetryErrorAction
ThrowWorkflowError: RetryErrorAction
SetVariableValue: RetryErrorAction
FailWorkflow: RetryErrorAction
Pause: HandlingStrategy
TearDown: HandlingStrategy

class StoreWorkflowRequest(_message.Message):
    __slots__ = ("workflow",)
    WORKFLOW_FIELD_NUMBER: _ClassVar[int]
    workflow: Workflow
    def __init__(self, workflow: _Optional[_Union[Workflow, _Mapping]] = ...) -> None: ...

class StoreWorkflowResponse(_message.Message):
    __slots__ = ("WorkflowId",)
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    WorkflowId: str
    def __init__(self, WorkflowId: _Optional[str] = ...) -> None: ...

class CancelProcessInstanceResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListWorkflowsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CompleteManualTaskResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CompleteServiceTaskResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CompleteUserTaskResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListUserTasksResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CompleteSendMessageResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetProcessInstanceStatusResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetTaskSpecUsageResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListExecutableProcessesRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListExecutableProcessesItem(_message.Message):
    __slots__ = ("processId", "workflowName", "parameter")
    PROCESSID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWNAME_FIELD_NUMBER: _ClassVar[int]
    PARAMETER_FIELD_NUMBER: _ClassVar[int]
    processId: str
    workflowName: str
    parameter: _containers.RepeatedCompositeFieldContainer[ExecutableStartParameter]
    def __init__(self, processId: _Optional[str] = ..., workflowName: _Optional[str] = ..., parameter: _Optional[_Iterable[_Union[ExecutableStartParameter, _Mapping]]] = ...) -> None: ...

class ExecutableStartParameter(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class HeartbeatRequest(_message.Message):
    __slots__ = ("host", "id", "time")
    HOST_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    host: str
    id: str
    time: int
    def __init__(self, host: _Optional[str] = ..., id: _Optional[str] = ..., time: _Optional[int] = ...) -> None: ...

class HeartbeatResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class Workflow(_message.Message):
    __slots__ = ("name", "process", "messages", "errors", "gzipSource", "collaboration", "messageReceivers")
    class ProcessEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Process
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Process, _Mapping]] = ...) -> None: ...
    class MessageReceiversEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: MessageReceivers
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[MessageReceivers, _Mapping]] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    PROCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGES_FIELD_NUMBER: _ClassVar[int]
    ERRORS_FIELD_NUMBER: _ClassVar[int]
    GZIPSOURCE_FIELD_NUMBER: _ClassVar[int]
    COLLABORATION_FIELD_NUMBER: _ClassVar[int]
    MESSAGERECEIVERS_FIELD_NUMBER: _ClassVar[int]
    name: str
    process: _containers.MessageMap[str, Process]
    messages: _containers.RepeatedCompositeFieldContainer[Element]
    errors: _containers.RepeatedCompositeFieldContainer[Error]
    gzipSource: bytes
    collaboration: Collaboration
    messageReceivers: _containers.MessageMap[str, MessageReceivers]
    def __init__(self, name: _Optional[str] = ..., process: _Optional[_Mapping[str, Process]] = ..., messages: _Optional[_Iterable[_Union[Element, _Mapping]]] = ..., errors: _Optional[_Iterable[_Union[Error, _Mapping]]] = ..., gzipSource: _Optional[bytes] = ..., collaboration: _Optional[_Union[Collaboration, _Mapping]] = ..., messageReceivers: _Optional[_Mapping[str, MessageReceivers]] = ...) -> None: ...

class MessageReceivers(_message.Message):
    __slots__ = ("messageReceiver", "associatedWorkflowName")
    MESSAGERECEIVER_FIELD_NUMBER: _ClassVar[int]
    ASSOCIATEDWORKFLOWNAME_FIELD_NUMBER: _ClassVar[int]
    messageReceiver: _containers.RepeatedCompositeFieldContainer[MessageReceiver]
    associatedWorkflowName: str
    def __init__(self, messageReceiver: _Optional[_Iterable[_Union[MessageReceiver, _Mapping]]] = ..., associatedWorkflowName: _Optional[str] = ...) -> None: ...

class MessageReceiver(_message.Message):
    __slots__ = ("id", "processIdToStart")
    ID_FIELD_NUMBER: _ClassVar[int]
    PROCESSIDTOSTART_FIELD_NUMBER: _ClassVar[int]
    id: str
    processIdToStart: str
    def __init__(self, id: _Optional[str] = ..., processIdToStart: _Optional[str] = ...) -> None: ...

class Collaboration(_message.Message):
    __slots__ = ("participant", "messageFlow")
    PARTICIPANT_FIELD_NUMBER: _ClassVar[int]
    MESSAGEFLOW_FIELD_NUMBER: _ClassVar[int]
    participant: _containers.RepeatedCompositeFieldContainer[Participant]
    messageFlow: _containers.RepeatedCompositeFieldContainer[MessageFlow]
    def __init__(self, participant: _Optional[_Iterable[_Union[Participant, _Mapping]]] = ..., messageFlow: _Optional[_Iterable[_Union[MessageFlow, _Mapping]]] = ...) -> None: ...

class Participant(_message.Message):
    __slots__ = ("id", "processId")
    ID_FIELD_NUMBER: _ClassVar[int]
    PROCESSID_FIELD_NUMBER: _ClassVar[int]
    id: str
    processId: str
    def __init__(self, id: _Optional[str] = ..., processId: _Optional[str] = ...) -> None: ...

class MessageFlow(_message.Message):
    __slots__ = ("id", "sender", "recipient")
    ID_FIELD_NUMBER: _ClassVar[int]
    SENDER_FIELD_NUMBER: _ClassVar[int]
    RECIPIENT_FIELD_NUMBER: _ClassVar[int]
    id: str
    sender: str
    recipient: str
    def __init__(self, id: _Optional[str] = ..., sender: _Optional[str] = ..., recipient: _Optional[str] = ...) -> None: ...

class Metadata(_message.Message):
    __slots__ = ("timedStart",)
    TIMEDSTART_FIELD_NUMBER: _ClassVar[int]
    timedStart: bool
    def __init__(self, timedStart: bool = ...) -> None: ...

class Process(_message.Message):
    __slots__ = ("elements", "Id", "metadata")
    ELEMENTS_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    elements: _containers.RepeatedCompositeFieldContainer[Element]
    Id: str
    metadata: Metadata
    def __init__(self, elements: _Optional[_Iterable[_Union[Element, _Mapping]]] = ..., Id: _Optional[str] = ..., metadata: _Optional[_Union[Metadata, _Mapping]] = ...) -> None: ...

class WorkflowVersions(_message.Message):
    __slots__ = ("version",)
    VERSION_FIELD_NUMBER: _ClassVar[int]
    version: _containers.RepeatedCompositeFieldContainer[WorkflowVersion]
    def __init__(self, version: _Optional[_Iterable[_Union[WorkflowVersion, _Mapping]]] = ...) -> None: ...

class WorkflowVersion(_message.Message):
    __slots__ = ("id", "sha256", "number")
    ID_FIELD_NUMBER: _ClassVar[int]
    SHA256_FIELD_NUMBER: _ClassVar[int]
    NUMBER_FIELD_NUMBER: _ClassVar[int]
    id: str
    sha256: bytes
    number: int
    def __init__(self, id: _Optional[str] = ..., sha256: _Optional[bytes] = ..., number: _Optional[int] = ...) -> None: ...

class Element(_message.Message):
    __slots__ = ("id", "name", "type", "documentation", "execute", "outbound", "compensateWith", "process", "msg", "candidates", "candidateGroups", "errors", "error", "inputTransform", "outputTransform", "timer", "boundaryTimer", "gateway", "iteration", "version", "retryBehaviour", "isForCompensation")
    class InputTransformEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    class OutputTransformEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    DOCUMENTATION_FIELD_NUMBER: _ClassVar[int]
    EXECUTE_FIELD_NUMBER: _ClassVar[int]
    OUTBOUND_FIELD_NUMBER: _ClassVar[int]
    COMPENSATEWITH_FIELD_NUMBER: _ClassVar[int]
    PROCESS_FIELD_NUMBER: _ClassVar[int]
    MSG_FIELD_NUMBER: _ClassVar[int]
    CANDIDATES_FIELD_NUMBER: _ClassVar[int]
    CANDIDATEGROUPS_FIELD_NUMBER: _ClassVar[int]
    ERRORS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    INPUTTRANSFORM_FIELD_NUMBER: _ClassVar[int]
    OUTPUTTRANSFORM_FIELD_NUMBER: _ClassVar[int]
    TIMER_FIELD_NUMBER: _ClassVar[int]
    BOUNDARYTIMER_FIELD_NUMBER: _ClassVar[int]
    GATEWAY_FIELD_NUMBER: _ClassVar[int]
    ITERATION_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    RETRYBEHAVIOUR_FIELD_NUMBER: _ClassVar[int]
    ISFORCOMPENSATION_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    type: str
    documentation: str
    execute: str
    outbound: Targets
    compensateWith: str
    process: Process
    msg: str
    candidates: str
    candidateGroups: str
    errors: _containers.RepeatedCompositeFieldContainer[CatchError]
    error: Error
    inputTransform: _containers.ScalarMap[str, str]
    outputTransform: _containers.ScalarMap[str, str]
    timer: WorkflowTimerDefinition
    boundaryTimer: _containers.RepeatedCompositeFieldContainer[Timer]
    gateway: GatewaySpec
    iteration: Iteration
    version: str
    retryBehaviour: DefaultTaskRetry
    isForCompensation: bool
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., type: _Optional[str] = ..., documentation: _Optional[str] = ..., execute: _Optional[str] = ..., outbound: _Optional[_Union[Targets, _Mapping]] = ..., compensateWith: _Optional[str] = ..., process: _Optional[_Union[Process, _Mapping]] = ..., msg: _Optional[str] = ..., candidates: _Optional[str] = ..., candidateGroups: _Optional[str] = ..., errors: _Optional[_Iterable[_Union[CatchError, _Mapping]]] = ..., error: _Optional[_Union[Error, _Mapping]] = ..., inputTransform: _Optional[_Mapping[str, str]] = ..., outputTransform: _Optional[_Mapping[str, str]] = ..., timer: _Optional[_Union[WorkflowTimerDefinition, _Mapping]] = ..., boundaryTimer: _Optional[_Iterable[_Union[Timer, _Mapping]]] = ..., gateway: _Optional[_Union[GatewaySpec, _Mapping]] = ..., iteration: _Optional[_Union[Iteration, _Mapping]] = ..., version: _Optional[str] = ..., retryBehaviour: _Optional[_Union[DefaultTaskRetry, _Mapping]] = ..., isForCompensation: bool = ...) -> None: ...

class Iteration(_message.Message):
    __slots__ = ("collection", "iterator", "collateAs", "collateFrom", "until", "execute")
    COLLECTION_FIELD_NUMBER: _ClassVar[int]
    ITERATOR_FIELD_NUMBER: _ClassVar[int]
    COLLATEAS_FIELD_NUMBER: _ClassVar[int]
    COLLATEFROM_FIELD_NUMBER: _ClassVar[int]
    UNTIL_FIELD_NUMBER: _ClassVar[int]
    EXECUTE_FIELD_NUMBER: _ClassVar[int]
    collection: str
    iterator: str
    collateAs: str
    collateFrom: str
    until: str
    execute: ThreadingType
    def __init__(self, collection: _Optional[str] = ..., iterator: _Optional[str] = ..., collateAs: _Optional[str] = ..., collateFrom: _Optional[str] = ..., until: _Optional[str] = ..., execute: _Optional[_Union[ThreadingType, str]] = ...) -> None: ...

class Iterator(_message.Message):
    __slots__ = ("id", "value", "collated")
    ID_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    COLLATED_FIELD_NUMBER: _ClassVar[int]
    id: str
    value: _containers.RepeatedScalarFieldContainer[bytes]
    collated: _containers.RepeatedScalarFieldContainer[bytes]
    def __init__(self, id: _Optional[str] = ..., value: _Optional[_Iterable[bytes]] = ..., collated: _Optional[_Iterable[bytes]] = ...) -> None: ...

class GatewaySpec(_message.Message):
    __slots__ = ("type", "direction", "reciprocalId", "fixedExpectations")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    DIRECTION_FIELD_NUMBER: _ClassVar[int]
    RECIPROCALID_FIELD_NUMBER: _ClassVar[int]
    FIXEDEXPECTATIONS_FIELD_NUMBER: _ClassVar[int]
    type: GatewayType
    direction: GatewayDirection
    reciprocalId: str
    fixedExpectations: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, type: _Optional[_Union[GatewayType, str]] = ..., direction: _Optional[_Union[GatewayDirection, str]] = ..., reciprocalId: _Optional[str] = ..., fixedExpectations: _Optional[_Iterable[str]] = ...) -> None: ...

class Timer(_message.Message):
    __slots__ = ("id", "duration", "target", "outputTransform")
    class OutputTransformEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    DURATION_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    OUTPUTTRANSFORM_FIELD_NUMBER: _ClassVar[int]
    id: str
    duration: str
    target: str
    outputTransform: _containers.ScalarMap[str, str]
    def __init__(self, id: _Optional[str] = ..., duration: _Optional[str] = ..., target: _Optional[str] = ..., outputTransform: _Optional[_Mapping[str, str]] = ...) -> None: ...

class Target(_message.Message):
    __slots__ = ("id", "conditions", "target")
    ID_FIELD_NUMBER: _ClassVar[int]
    CONDITIONS_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    id: str
    conditions: _containers.RepeatedScalarFieldContainer[str]
    target: str
    def __init__(self, id: _Optional[str] = ..., conditions: _Optional[_Iterable[str]] = ..., target: _Optional[str] = ...) -> None: ...

class Error(_message.Message):
    __slots__ = ("id", "name", "code")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    code: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ..., code: _Optional[str] = ...) -> None: ...

class CatchError(_message.Message):
    __slots__ = ("id", "errorId", "target", "outputTransform")
    class OutputTransformEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    ID_FIELD_NUMBER: _ClassVar[int]
    ERRORID_FIELD_NUMBER: _ClassVar[int]
    TARGET_FIELD_NUMBER: _ClassVar[int]
    OUTPUTTRANSFORM_FIELD_NUMBER: _ClassVar[int]
    id: str
    errorId: str
    target: str
    outputTransform: _containers.ScalarMap[str, str]
    def __init__(self, id: _Optional[str] = ..., errorId: _Optional[str] = ..., target: _Optional[str] = ..., outputTransform: _Optional[_Mapping[str, str]] = ...) -> None: ...

class Targets(_message.Message):
    __slots__ = ("target", "defaultTarget")
    TARGET_FIELD_NUMBER: _ClassVar[int]
    DEFAULTTARGET_FIELD_NUMBER: _ClassVar[int]
    target: _containers.RepeatedCompositeFieldContainer[Target]
    defaultTarget: int
    def __init__(self, target: _Optional[_Iterable[_Union[Target, _Mapping]]] = ..., defaultTarget: _Optional[int] = ...) -> None: ...

class WorkflowState(_message.Message):
    __slots__ = ("workflowId", "executionId", "elementId", "elementType", "id", "executeVersion", "execute", "state", "condition", "unixTimeNano", "vars", "owners", "groups", "error", "timer", "workflowName", "processId", "processInstanceId", "satisfiesGatewayExpectation", "gatewayExpectations", "traceParent", "compensation", "elementName")
    class SatisfiesGatewayExpectationEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: SatisfiesGateway
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[SatisfiesGateway, _Mapping]] = ...) -> None: ...
    class GatewayExpectationsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: GatewayExpectations
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[GatewayExpectations, _Mapping]] = ...) -> None: ...
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    EXECUTIONID_FIELD_NUMBER: _ClassVar[int]
    ELEMENTID_FIELD_NUMBER: _ClassVar[int]
    ELEMENTTYPE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    EXECUTEVERSION_FIELD_NUMBER: _ClassVar[int]
    EXECUTE_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    CONDITION_FIELD_NUMBER: _ClassVar[int]
    UNIXTIMENANO_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    OWNERS_FIELD_NUMBER: _ClassVar[int]
    GROUPS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    TIMER_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWNAME_FIELD_NUMBER: _ClassVar[int]
    PROCESSID_FIELD_NUMBER: _ClassVar[int]
    PROCESSINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    SATISFIESGATEWAYEXPECTATION_FIELD_NUMBER: _ClassVar[int]
    GATEWAYEXPECTATIONS_FIELD_NUMBER: _ClassVar[int]
    TRACEPARENT_FIELD_NUMBER: _ClassVar[int]
    COMPENSATION_FIELD_NUMBER: _ClassVar[int]
    ELEMENTNAME_FIELD_NUMBER: _ClassVar[int]
    workflowId: str
    executionId: str
    elementId: str
    elementType: str
    id: _containers.RepeatedScalarFieldContainer[str]
    executeVersion: str
    execute: str
    state: CancellationState
    condition: str
    unixTimeNano: int
    vars: bytes
    owners: _containers.RepeatedScalarFieldContainer[str]
    groups: _containers.RepeatedScalarFieldContainer[str]
    error: Error
    timer: WorkflowTimer
    workflowName: str
    processId: str
    processInstanceId: str
    satisfiesGatewayExpectation: _containers.MessageMap[str, SatisfiesGateway]
    gatewayExpectations: _containers.MessageMap[str, GatewayExpectations]
    traceParent: str
    compensation: Compensation
    elementName: str
    def __init__(self, workflowId: _Optional[str] = ..., executionId: _Optional[str] = ..., elementId: _Optional[str] = ..., elementType: _Optional[str] = ..., id: _Optional[_Iterable[str]] = ..., executeVersion: _Optional[str] = ..., execute: _Optional[str] = ..., state: _Optional[_Union[CancellationState, str]] = ..., condition: _Optional[str] = ..., unixTimeNano: _Optional[int] = ..., vars: _Optional[bytes] = ..., owners: _Optional[_Iterable[str]] = ..., groups: _Optional[_Iterable[str]] = ..., error: _Optional[_Union[Error, _Mapping]] = ..., timer: _Optional[_Union[WorkflowTimer, _Mapping]] = ..., workflowName: _Optional[str] = ..., processId: _Optional[str] = ..., processInstanceId: _Optional[str] = ..., satisfiesGatewayExpectation: _Optional[_Mapping[str, SatisfiesGateway]] = ..., gatewayExpectations: _Optional[_Mapping[str, GatewayExpectations]] = ..., traceParent: _Optional[str] = ..., compensation: _Optional[_Union[Compensation, _Mapping]] = ..., elementName: _Optional[str] = ...) -> None: ...

class Compensation(_message.Message):
    __slots__ = ("step", "totalSteps", "forTrackingId")
    STEP_FIELD_NUMBER: _ClassVar[int]
    TOTALSTEPS_FIELD_NUMBER: _ClassVar[int]
    FORTRACKINGID_FIELD_NUMBER: _ClassVar[int]
    step: int
    totalSteps: int
    forTrackingId: str
    def __init__(self, step: _Optional[int] = ..., totalSteps: _Optional[int] = ..., forTrackingId: _Optional[str] = ...) -> None: ...

class WorkflowStateSummary(_message.Message):
    __slots__ = ("workflowId", "executionId", "elementId", "elementType", "id", "execute", "state", "condition", "unixTimeNano", "vars", "error", "timer", "processInstanceId")
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    EXECUTIONID_FIELD_NUMBER: _ClassVar[int]
    ELEMENTID_FIELD_NUMBER: _ClassVar[int]
    ELEMENTTYPE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    EXECUTE_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    CONDITION_FIELD_NUMBER: _ClassVar[int]
    UNIXTIMENANO_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    TIMER_FIELD_NUMBER: _ClassVar[int]
    PROCESSINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    workflowId: str
    executionId: str
    elementId: str
    elementType: str
    id: _containers.RepeatedScalarFieldContainer[str]
    execute: str
    state: CancellationState
    condition: str
    unixTimeNano: int
    vars: bytes
    error: Error
    timer: WorkflowTimer
    processInstanceId: str
    def __init__(self, workflowId: _Optional[str] = ..., executionId: _Optional[str] = ..., elementId: _Optional[str] = ..., elementType: _Optional[str] = ..., id: _Optional[_Iterable[str]] = ..., execute: _Optional[str] = ..., state: _Optional[_Union[CancellationState, str]] = ..., condition: _Optional[str] = ..., unixTimeNano: _Optional[int] = ..., vars: _Optional[bytes] = ..., error: _Optional[_Union[Error, _Mapping]] = ..., timer: _Optional[_Union[WorkflowTimer, _Mapping]] = ..., processInstanceId: _Optional[str] = ...) -> None: ...

class ProcessHistoryEntry(_message.Message):
    __slots__ = ("itemType", "workflowId", "executionId", "elementId", "elementName", "processInstanceId", "cancellationState", "vars", "timer", "error", "unixTimeNano", "execute", "id", "compensating", "processId", "satisfiesGatewayExpectation", "gatewayExpectations", "workflowName")
    class SatisfiesGatewayExpectationEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: SatisfiesGateway
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[SatisfiesGateway, _Mapping]] = ...) -> None: ...
    class GatewayExpectationsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: GatewayExpectations
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[GatewayExpectations, _Mapping]] = ...) -> None: ...
    ITEMTYPE_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    EXECUTIONID_FIELD_NUMBER: _ClassVar[int]
    ELEMENTID_FIELD_NUMBER: _ClassVar[int]
    ELEMENTNAME_FIELD_NUMBER: _ClassVar[int]
    PROCESSINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    CANCELLATIONSTATE_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    TIMER_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    UNIXTIMENANO_FIELD_NUMBER: _ClassVar[int]
    EXECUTE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    COMPENSATING_FIELD_NUMBER: _ClassVar[int]
    PROCESSID_FIELD_NUMBER: _ClassVar[int]
    SATISFIESGATEWAYEXPECTATION_FIELD_NUMBER: _ClassVar[int]
    GATEWAYEXPECTATIONS_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWNAME_FIELD_NUMBER: _ClassVar[int]
    itemType: ProcessHistoryType
    workflowId: str
    executionId: str
    elementId: str
    elementName: str
    processInstanceId: str
    cancellationState: CancellationState
    vars: bytes
    timer: WorkflowTimer
    error: Error
    unixTimeNano: int
    execute: str
    id: _containers.RepeatedScalarFieldContainer[str]
    compensating: bool
    processId: str
    satisfiesGatewayExpectation: _containers.MessageMap[str, SatisfiesGateway]
    gatewayExpectations: _containers.MessageMap[str, GatewayExpectations]
    workflowName: str
    def __init__(self, itemType: _Optional[_Union[ProcessHistoryType, str]] = ..., workflowId: _Optional[str] = ..., executionId: _Optional[str] = ..., elementId: _Optional[str] = ..., elementName: _Optional[str] = ..., processInstanceId: _Optional[str] = ..., cancellationState: _Optional[_Union[CancellationState, str]] = ..., vars: _Optional[bytes] = ..., timer: _Optional[_Union[WorkflowTimer, _Mapping]] = ..., error: _Optional[_Union[Error, _Mapping]] = ..., unixTimeNano: _Optional[int] = ..., execute: _Optional[str] = ..., id: _Optional[_Iterable[str]] = ..., compensating: bool = ..., processId: _Optional[str] = ..., satisfiesGatewayExpectation: _Optional[_Mapping[str, SatisfiesGateway]] = ..., gatewayExpectations: _Optional[_Mapping[str, GatewayExpectations]] = ..., workflowName: _Optional[str] = ...) -> None: ...

class ProcessHistory(_message.Message):
    __slots__ = ("item",)
    ITEM_FIELD_NUMBER: _ClassVar[int]
    item: _containers.RepeatedCompositeFieldContainer[ProcessHistoryEntry]
    def __init__(self, item: _Optional[_Iterable[_Union[ProcessHistoryEntry, _Mapping]]] = ...) -> None: ...

class SatisfiesGateway(_message.Message):
    __slots__ = ("instanceTracking",)
    INSTANCETRACKING_FIELD_NUMBER: _ClassVar[int]
    instanceTracking: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, instanceTracking: _Optional[_Iterable[str]] = ...) -> None: ...

class GatewayExpectations(_message.Message):
    __slots__ = ("expectedPaths",)
    EXPECTEDPATHS_FIELD_NUMBER: _ClassVar[int]
    expectedPaths: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, expectedPaths: _Optional[_Iterable[str]] = ...) -> None: ...

class Gateway(_message.Message):
    __slots__ = ("metExpectations", "vars", "visits")
    class MetExpectationsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    METEXPECTATIONS_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    VISITS_FIELD_NUMBER: _ClassVar[int]
    metExpectations: _containers.ScalarMap[str, str]
    vars: _containers.RepeatedScalarFieldContainer[bytes]
    visits: int
    def __init__(self, metExpectations: _Optional[_Mapping[str, str]] = ..., vars: _Optional[_Iterable[bytes]] = ..., visits: _Optional[int] = ...) -> None: ...

class WorkflowTimerDefinition(_message.Message):
    __slots__ = ("type", "value", "repeat", "dropEvents")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    VALUE_FIELD_NUMBER: _ClassVar[int]
    REPEAT_FIELD_NUMBER: _ClassVar[int]
    DROPEVENTS_FIELD_NUMBER: _ClassVar[int]
    type: WorkflowTimerType
    value: int
    repeat: int
    dropEvents: bool
    def __init__(self, type: _Optional[_Union[WorkflowTimerType, str]] = ..., value: _Optional[int] = ..., repeat: _Optional[int] = ..., dropEvents: bool = ...) -> None: ...

class WorkflowTimer(_message.Message):
    __slots__ = ("lastFired", "count")
    LASTFIRED_FIELD_NUMBER: _ClassVar[int]
    COUNT_FIELD_NUMBER: _ClassVar[int]
    lastFired: int
    count: int
    def __init__(self, lastFired: _Optional[int] = ..., count: _Optional[int] = ...) -> None: ...

class Execution(_message.Message):
    __slots__ = ("executionId", "workflowId", "workflowName", "processInstanceId")
    EXECUTIONID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWNAME_FIELD_NUMBER: _ClassVar[int]
    PROCESSINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    executionId: str
    workflowId: str
    workflowName: str
    processInstanceId: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, executionId: _Optional[str] = ..., workflowId: _Optional[str] = ..., workflowName: _Optional[str] = ..., processInstanceId: _Optional[_Iterable[str]] = ...) -> None: ...

class ProcessInstance(_message.Message):
    __slots__ = ("processInstanceId", "executionId", "parentProcessId", "parentElementId", "workflowId", "workflowName", "processId", "gatewayComplete")
    class GatewayCompleteEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: bool
        def __init__(self, key: _Optional[str] = ..., value: bool = ...) -> None: ...
    PROCESSINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    EXECUTIONID_FIELD_NUMBER: _ClassVar[int]
    PARENTPROCESSID_FIELD_NUMBER: _ClassVar[int]
    PARENTELEMENTID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWNAME_FIELD_NUMBER: _ClassVar[int]
    PROCESSID_FIELD_NUMBER: _ClassVar[int]
    GATEWAYCOMPLETE_FIELD_NUMBER: _ClassVar[int]
    processInstanceId: str
    executionId: str
    parentProcessId: str
    parentElementId: str
    workflowId: str
    workflowName: str
    processId: str
    gatewayComplete: _containers.ScalarMap[str, bool]
    def __init__(self, processInstanceId: _Optional[str] = ..., executionId: _Optional[str] = ..., parentProcessId: _Optional[str] = ..., parentElementId: _Optional[str] = ..., workflowId: _Optional[str] = ..., workflowName: _Optional[str] = ..., processId: _Optional[str] = ..., gatewayComplete: _Optional[_Mapping[str, bool]] = ...) -> None: ...

class MessageInstance(_message.Message):
    __slots__ = ("name", "correlationKey", "vars")
    NAME_FIELD_NUMBER: _ClassVar[int]
    CORRELATIONKEY_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    name: str
    correlationKey: str
    vars: bytes
    def __init__(self, name: _Optional[str] = ..., correlationKey: _Optional[str] = ..., vars: _Optional[bytes] = ...) -> None: ...

class MessageRecipient(_message.Message):
    __slots__ = ("type", "id", "CorrelationKey")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    ID_FIELD_NUMBER: _ClassVar[int]
    CORRELATIONKEY_FIELD_NUMBER: _ClassVar[int]
    type: RecipientType
    id: str
    CorrelationKey: str
    def __init__(self, type: _Optional[_Union[RecipientType, str]] = ..., id: _Optional[str] = ..., CorrelationKey: _Optional[str] = ...) -> None: ...

class UserTasks(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[_Iterable[str]] = ...) -> None: ...

class ApiAuthorizationRequest(_message.Message):
    __slots__ = ("Headers", "function", "workflowName", "user")
    class HeadersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    FUNCTION_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWNAME_FIELD_NUMBER: _ClassVar[int]
    USER_FIELD_NUMBER: _ClassVar[int]
    Headers: _containers.ScalarMap[str, str]
    function: str
    workflowName: str
    user: str
    def __init__(self, Headers: _Optional[_Mapping[str, str]] = ..., function: _Optional[str] = ..., workflowName: _Optional[str] = ..., user: _Optional[str] = ...) -> None: ...

class ApiAuthorizationResponse(_message.Message):
    __slots__ = ("authorized", "userId")
    AUTHORIZED_FIELD_NUMBER: _ClassVar[int]
    USERID_FIELD_NUMBER: _ClassVar[int]
    authorized: bool
    userId: str
    def __init__(self, authorized: bool = ..., userId: _Optional[str] = ...) -> None: ...

class ApiAuthenticationRequest(_message.Message):
    __slots__ = ("headers",)
    class HeadersEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    HEADERS_FIELD_NUMBER: _ClassVar[int]
    headers: _containers.ScalarMap[str, str]
    def __init__(self, headers: _Optional[_Mapping[str, str]] = ...) -> None: ...

class ApiAuthenticationResponse(_message.Message):
    __slots__ = ("user", "Authenticated")
    USER_FIELD_NUMBER: _ClassVar[int]
    AUTHENTICATED_FIELD_NUMBER: _ClassVar[int]
    user: str
    Authenticated: bool
    def __init__(self, user: _Optional[str] = ..., Authenticated: bool = ...) -> None: ...

class LaunchWorkflowRequest(_message.Message):
    __slots__ = ("processId", "vars")
    PROCESSID_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    processId: str
    vars: bytes
    def __init__(self, processId: _Optional[str] = ..., vars: _Optional[bytes] = ...) -> None: ...

class LaunchWorkflowResponse(_message.Message):
    __slots__ = ("executionId", "workflowId")
    EXECUTIONID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    executionId: str
    workflowId: str
    def __init__(self, executionId: _Optional[str] = ..., workflowId: _Optional[str] = ...) -> None: ...

class CancelProcessInstanceRequest(_message.Message):
    __slots__ = ("id", "state", "error")
    ID_FIELD_NUMBER: _ClassVar[int]
    STATE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    id: str
    state: CancellationState
    error: Error
    def __init__(self, id: _Optional[str] = ..., state: _Optional[_Union[CancellationState, str]] = ..., error: _Optional[_Union[Error, _Mapping]] = ...) -> None: ...

class ListExecutionProcessesRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class ListExecutionProcessesResponse(_message.Message):
    __slots__ = ("processInstanceId",)
    PROCESSINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    processInstanceId: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, processInstanceId: _Optional[_Iterable[str]] = ...) -> None: ...

class GetProcessInstanceStatusRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetProcessInstanceStatusResult(_message.Message):
    __slots__ = ("processState",)
    PROCESSSTATE_FIELD_NUMBER: _ClassVar[int]
    processState: _containers.RepeatedCompositeFieldContainer[WorkflowState]
    def __init__(self, processState: _Optional[_Iterable[_Union[WorkflowState, _Mapping]]] = ...) -> None: ...

class ListExecutionRequest(_message.Message):
    __slots__ = ("workflowName",)
    WORKFLOWNAME_FIELD_NUMBER: _ClassVar[int]
    workflowName: str
    def __init__(self, workflowName: _Optional[str] = ...) -> None: ...

class ListExecutionResponse(_message.Message):
    __slots__ = ("result",)
    RESULT_FIELD_NUMBER: _ClassVar[int]
    result: _containers.RepeatedCompositeFieldContainer[ListExecutionItem]
    def __init__(self, result: _Optional[_Iterable[_Union[ListExecutionItem, _Mapping]]] = ...) -> None: ...

class ListExecutionItem(_message.Message):
    __slots__ = ("id", "version")
    ID_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    id: str
    version: int
    def __init__(self, id: _Optional[str] = ..., version: _Optional[int] = ...) -> None: ...

class WorkflowInstanceInfo(_message.Message):
    __slots__ = ("id", "workflowId")
    ID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    id: str
    workflowId: str
    def __init__(self, id: _Optional[str] = ..., workflowId: _Optional[str] = ...) -> None: ...

class WorkflowInstanceStatus(_message.Message):
    __slots__ = ("state",)
    STATE_FIELD_NUMBER: _ClassVar[int]
    state: _containers.RepeatedCompositeFieldContainer[WorkflowState]
    def __init__(self, state: _Optional[_Iterable[_Union[WorkflowState, _Mapping]]] = ...) -> None: ...

class ListWorkflowsResponse(_message.Message):
    __slots__ = ("result",)
    RESULT_FIELD_NUMBER: _ClassVar[int]
    result: _containers.RepeatedCompositeFieldContainer[ListWorkflowResponse]
    def __init__(self, result: _Optional[_Iterable[_Union[ListWorkflowResponse, _Mapping]]] = ...) -> None: ...

class ListWorkflowResponse(_message.Message):
    __slots__ = ("name", "version")
    NAME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    name: str
    version: int
    def __init__(self, name: _Optional[str] = ..., version: _Optional[int] = ...) -> None: ...

class SendMessageRequest(_message.Message):
    __slots__ = ("name", "correlationKey", "vars")
    NAME_FIELD_NUMBER: _ClassVar[int]
    CORRELATIONKEY_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    name: str
    correlationKey: str
    vars: bytes
    def __init__(self, name: _Optional[str] = ..., correlationKey: _Optional[str] = ..., vars: _Optional[bytes] = ...) -> None: ...

class SendMessageResponse(_message.Message):
    __slots__ = ("executionId", "workflowId")
    EXECUTIONID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    executionId: str
    workflowId: str
    def __init__(self, executionId: _Optional[str] = ..., workflowId: _Optional[str] = ...) -> None: ...

class WorkflowInstanceComplete(_message.Message):
    __slots__ = ("workflowName", "workflowId", "workflowInstanceId", "workflowState", "error")
    WORKFLOWNAME_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWSTATE_FIELD_NUMBER: _ClassVar[int]
    ERROR_FIELD_NUMBER: _ClassVar[int]
    workflowName: str
    workflowId: str
    workflowInstanceId: str
    workflowState: CancellationState
    error: Error
    def __init__(self, workflowName: _Optional[str] = ..., workflowId: _Optional[str] = ..., workflowInstanceId: _Optional[str] = ..., workflowState: _Optional[_Union[CancellationState, str]] = ..., error: _Optional[_Union[Error, _Mapping]] = ...) -> None: ...

class CompleteManualTaskRequest(_message.Message):
    __slots__ = ("trackingId", "vars")
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    trackingId: str
    vars: bytes
    def __init__(self, trackingId: _Optional[str] = ..., vars: _Optional[bytes] = ...) -> None: ...

class CompleteServiceTaskRequest(_message.Message):
    __slots__ = ("trackingId", "vars", "compensating")
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    COMPENSATING_FIELD_NUMBER: _ClassVar[int]
    trackingId: str
    vars: bytes
    compensating: bool
    def __init__(self, trackingId: _Optional[str] = ..., vars: _Optional[bytes] = ..., compensating: bool = ...) -> None: ...

class CompleteSendMessageRequest(_message.Message):
    __slots__ = ("trackingId", "vars")
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    trackingId: str
    vars: bytes
    def __init__(self, trackingId: _Optional[str] = ..., vars: _Optional[bytes] = ...) -> None: ...

class CompleteUserTaskRequest(_message.Message):
    __slots__ = ("trackingId", "owner", "vars")
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    OWNER_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    trackingId: str
    owner: str
    vars: bytes
    def __init__(self, trackingId: _Optional[str] = ..., owner: _Optional[str] = ..., vars: _Optional[bytes] = ...) -> None: ...

class ListUserTasksRequest(_message.Message):
    __slots__ = ("owner",)
    OWNER_FIELD_NUMBER: _ClassVar[int]
    owner: str
    def __init__(self, owner: _Optional[str] = ...) -> None: ...

class GetUserTaskRequest(_message.Message):
    __slots__ = ("owner", "trackingId")
    OWNER_FIELD_NUMBER: _ClassVar[int]
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    owner: str
    trackingId: str
    def __init__(self, owner: _Optional[str] = ..., trackingId: _Optional[str] = ...) -> None: ...

class GetUserTaskResponse(_message.Message):
    __slots__ = ("trackingId", "owner", "description", "name", "vars")
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    OWNER_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    trackingId: str
    owner: str
    description: str
    name: str
    vars: bytes
    def __init__(self, trackingId: _Optional[str] = ..., owner: _Optional[str] = ..., description: _Optional[str] = ..., name: _Optional[str] = ..., vars: _Optional[bytes] = ...) -> None: ...

class HandleWorkflowErrorRequest(_message.Message):
    __slots__ = ("trackingId", "errorCode", "message", "vars")
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    ERRORCODE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    VARS_FIELD_NUMBER: _ClassVar[int]
    trackingId: str
    errorCode: str
    message: str
    vars: bytes
    def __init__(self, trackingId: _Optional[str] = ..., errorCode: _Optional[str] = ..., message: _Optional[str] = ..., vars: _Optional[bytes] = ...) -> None: ...

class HandleWorkflowErrorResponse(_message.Message):
    __slots__ = ("handled",)
    HANDLED_FIELD_NUMBER: _ClassVar[int]
    handled: bool
    def __init__(self, handled: bool = ...) -> None: ...

class HandleWorkflowFatalErrorRequest(_message.Message):
    __slots__ = ("workflowState", "message")
    WORKFLOWSTATE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    workflowState: WorkflowState
    message: str
    def __init__(self, workflowState: _Optional[_Union[WorkflowState, _Mapping]] = ..., message: _Optional[str] = ...) -> None: ...

class HandleWorkflowFatalErrorResponse(_message.Message):
    __slots__ = ("handled",)
    HANDLED_FIELD_NUMBER: _ClassVar[int]
    handled: bool
    def __init__(self, handled: bool = ...) -> None: ...

class GetWorkflowVersionsRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class GetWorkflowVersionsResponse(_message.Message):
    __slots__ = ("versions",)
    VERSIONS_FIELD_NUMBER: _ClassVar[int]
    versions: WorkflowVersions
    def __init__(self, versions: _Optional[_Union[WorkflowVersions, _Mapping]] = ...) -> None: ...

class GetWorkflowRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetWorkflowResponse(_message.Message):
    __slots__ = ("definition",)
    DEFINITION_FIELD_NUMBER: _ClassVar[int]
    definition: Workflow
    def __init__(self, definition: _Optional[_Union[Workflow, _Mapping]] = ...) -> None: ...

class GetProcessHistoryRequest(_message.Message):
    __slots__ = ("Id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    Id: str
    def __init__(self, Id: _Optional[str] = ...) -> None: ...

class GetProcessHistoryResponse(_message.Message):
    __slots__ = ("entry",)
    ENTRY_FIELD_NUMBER: _ClassVar[int]
    entry: _containers.RepeatedCompositeFieldContainer[ProcessHistoryEntry]
    def __init__(self, entry: _Optional[_Iterable[_Union[ProcessHistoryEntry, _Mapping]]] = ...) -> None: ...

class GetServiceTaskRoutingIDRequest(_message.Message):
    __slots__ = ("name", "requestedId")
    NAME_FIELD_NUMBER: _ClassVar[int]
    REQUESTEDID_FIELD_NUMBER: _ClassVar[int]
    name: str
    requestedId: str
    def __init__(self, name: _Optional[str] = ..., requestedId: _Optional[str] = ...) -> None: ...

class GetServiceTaskRoutingIDResponse(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class GetVersionInfoRequest(_message.Message):
    __slots__ = ("clientVersion", "compatibleVersion")
    CLIENTVERSION_FIELD_NUMBER: _ClassVar[int]
    COMPATIBLEVERSION_FIELD_NUMBER: _ClassVar[int]
    clientVersion: str
    compatibleVersion: str
    def __init__(self, clientVersion: _Optional[str] = ..., compatibleVersion: _Optional[str] = ...) -> None: ...

class GetVersionInfoResponse(_message.Message):
    __slots__ = ("serverVersion", "minCompatibleVersion", "connect")
    SERVERVERSION_FIELD_NUMBER: _ClassVar[int]
    MINCOMPATIBLEVERSION_FIELD_NUMBER: _ClassVar[int]
    CONNECT_FIELD_NUMBER: _ClassVar[int]
    serverVersion: str
    minCompatibleVersion: str
    connect: bool
    def __init__(self, serverVersion: _Optional[str] = ..., minCompatibleVersion: _Optional[str] = ..., connect: bool = ...) -> None: ...

class GetJobRequest(_message.Message):
    __slots__ = ("jobId",)
    JOBID_FIELD_NUMBER: _ClassVar[int]
    jobId: str
    def __init__(self, jobId: _Optional[str] = ...) -> None: ...

class GetJobResponse(_message.Message):
    __slots__ = ("job",)
    JOB_FIELD_NUMBER: _ClassVar[int]
    job: WorkflowState
    def __init__(self, job: _Optional[_Union[WorkflowState, _Mapping]] = ...) -> None: ...

class ResolveWorkflowRequest(_message.Message):
    __slots__ = ("workflow",)
    WORKFLOW_FIELD_NUMBER: _ClassVar[int]
    workflow: Workflow
    def __init__(self, workflow: _Optional[_Union[Workflow, _Mapping]] = ...) -> None: ...

class ResolveWorkflowResponse(_message.Message):
    __slots__ = ("workflow",)
    WORKFLOW_FIELD_NUMBER: _ClassVar[int]
    workflow: Workflow
    def __init__(self, workflow: _Optional[_Union[Workflow, _Mapping]] = ...) -> None: ...

class WorkflowStats(_message.Message):
    __slots__ = ("Workflows", "InstancesStarted", "InstancesComplete")
    WORKFLOWS_FIELD_NUMBER: _ClassVar[int]
    INSTANCESSTARTED_FIELD_NUMBER: _ClassVar[int]
    INSTANCESCOMPLETE_FIELD_NUMBER: _ClassVar[int]
    Workflows: int
    InstancesStarted: int
    InstancesComplete: int
    def __init__(self, Workflows: _Optional[int] = ..., InstancesStarted: _Optional[int] = ..., InstancesComplete: _Optional[int] = ...) -> None: ...

class TelemetryState(_message.Message):
    __slots__ = ("state", "log")
    class LogEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: int
        value: TelemetryLogEntry
        def __init__(self, key: _Optional[int] = ..., value: _Optional[_Union[TelemetryLogEntry, _Mapping]] = ...) -> None: ...
    STATE_FIELD_NUMBER: _ClassVar[int]
    LOG_FIELD_NUMBER: _ClassVar[int]
    state: WorkflowState
    log: _containers.MessageMap[int, TelemetryLogEntry]
    def __init__(self, state: _Optional[_Union[WorkflowState, _Mapping]] = ..., log: _Optional[_Mapping[int, TelemetryLogEntry]] = ...) -> None: ...

class TelemetryLogEntry(_message.Message):
    __slots__ = ("trackingID", "source", "message", "code", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    trackingID: str
    source: LogSource
    message: str
    code: int
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, trackingID: _Optional[str] = ..., source: _Optional[_Union[LogSource, str]] = ..., message: _Optional[str] = ..., code: _Optional[int] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class TaskSpecVersions(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[_Iterable[str]] = ...) -> None: ...

class TaskSpec(_message.Message):
    __slots__ = ("version", "kind", "metadata", "behaviour", "parameters", "events")
    VERSION_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    BEHAVIOUR_FIELD_NUMBER: _ClassVar[int]
    PARAMETERS_FIELD_NUMBER: _ClassVar[int]
    EVENTS_FIELD_NUMBER: _ClassVar[int]
    version: str
    kind: str
    metadata: TaskMetadata
    behaviour: TaskBehaviour
    parameters: TaskParameters
    events: TaskEvents
    def __init__(self, version: _Optional[str] = ..., kind: _Optional[str] = ..., metadata: _Optional[_Union[TaskMetadata, _Mapping]] = ..., behaviour: _Optional[_Union[TaskBehaviour, _Mapping]] = ..., parameters: _Optional[_Union[TaskParameters, _Mapping]] = ..., events: _Optional[_Union[TaskEvents, _Mapping]] = ...) -> None: ...

class TaskMetadata(_message.Message):
    __slots__ = ("uid", "type", "version", "short", "description", "labels", "extensionData")
    class ExtensionDataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    UID_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    SHORT_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    LABELS_FIELD_NUMBER: _ClassVar[int]
    EXTENSIONDATA_FIELD_NUMBER: _ClassVar[int]
    uid: str
    type: str
    version: str
    short: str
    description: str
    labels: _containers.RepeatedScalarFieldContainer[str]
    extensionData: _containers.ScalarMap[str, str]
    def __init__(self, uid: _Optional[str] = ..., type: _Optional[str] = ..., version: _Optional[str] = ..., short: _Optional[str] = ..., description: _Optional[str] = ..., labels: _Optional[_Iterable[str]] = ..., extensionData: _Optional[_Mapping[str, str]] = ...) -> None: ...

class TaskParameters(_message.Message):
    __slots__ = ("parameterGroup", "input", "output")
    PARAMETERGROUP_FIELD_NUMBER: _ClassVar[int]
    INPUT_FIELD_NUMBER: _ClassVar[int]
    OUTPUT_FIELD_NUMBER: _ClassVar[int]
    parameterGroup: _containers.RepeatedCompositeFieldContainer[ParameterGroup]
    input: _containers.RepeatedCompositeFieldContainer[Parameter]
    output: _containers.RepeatedCompositeFieldContainer[Parameter]
    def __init__(self, parameterGroup: _Optional[_Iterable[_Union[ParameterGroup, _Mapping]]] = ..., input: _Optional[_Iterable[_Union[Parameter, _Mapping]]] = ..., output: _Optional[_Iterable[_Union[Parameter, _Mapping]]] = ...) -> None: ...

class TaskEvents(_message.Message):
    __slots__ = ("error", "message")
    ERROR_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    error: _containers.RepeatedCompositeFieldContainer[TaskError]
    message: _containers.RepeatedCompositeFieldContainer[Message]
    def __init__(self, error: _Optional[_Iterable[_Union[TaskError, _Mapping]]] = ..., message: _Optional[_Iterable[_Union[Message, _Mapping]]] = ...) -> None: ...

class TaskBehaviour(_message.Message):
    __slots__ = ("defaultRetry", "estimatedMaxDuration", "unsafe", "mock", "deprecated", "mockBehaviour")
    DEFAULTRETRY_FIELD_NUMBER: _ClassVar[int]
    ESTIMATEDMAXDURATION_FIELD_NUMBER: _ClassVar[int]
    UNSAFE_FIELD_NUMBER: _ClassVar[int]
    MOCK_FIELD_NUMBER: _ClassVar[int]
    DEPRECATED_FIELD_NUMBER: _ClassVar[int]
    MOCKBEHAVIOUR_FIELD_NUMBER: _ClassVar[int]
    defaultRetry: DefaultTaskRetry
    estimatedMaxDuration: int
    unsafe: bool
    mock: bool
    deprecated: bool
    mockBehaviour: TaskMockBehaviours
    def __init__(self, defaultRetry: _Optional[_Union[DefaultTaskRetry, _Mapping]] = ..., estimatedMaxDuration: _Optional[int] = ..., unsafe: bool = ..., mock: bool = ..., deprecated: bool = ..., mockBehaviour: _Optional[_Union[TaskMockBehaviours, _Mapping]] = ...) -> None: ...

class TaskMockBehaviours(_message.Message):
    __slots__ = ("errorCodeExpr", "fatalErrorExpr")
    ERRORCODEEXPR_FIELD_NUMBER: _ClassVar[int]
    FATALERROREXPR_FIELD_NUMBER: _ClassVar[int]
    errorCodeExpr: str
    fatalErrorExpr: str
    def __init__(self, errorCodeExpr: _Optional[str] = ..., fatalErrorExpr: _Optional[str] = ...) -> None: ...

class DefaultTaskRetry(_message.Message):
    __slots__ = ("number", "strategy", "initMilli", "intervalMilli", "maxMilli", "defaultExceeded")
    NUMBER_FIELD_NUMBER: _ClassVar[int]
    STRATEGY_FIELD_NUMBER: _ClassVar[int]
    INITMILLI_FIELD_NUMBER: _ClassVar[int]
    INTERVALMILLI_FIELD_NUMBER: _ClassVar[int]
    MAXMILLI_FIELD_NUMBER: _ClassVar[int]
    DEFAULTEXCEEDED_FIELD_NUMBER: _ClassVar[int]
    number: int
    strategy: RetryStrategy
    initMilli: int
    intervalMilli: int
    maxMilli: int
    defaultExceeded: DefaultRetryExceededBehaviour
    def __init__(self, number: _Optional[int] = ..., strategy: _Optional[_Union[RetryStrategy, str]] = ..., initMilli: _Optional[int] = ..., intervalMilli: _Optional[int] = ..., maxMilli: _Optional[int] = ..., defaultExceeded: _Optional[_Union[DefaultRetryExceededBehaviour, _Mapping]] = ...) -> None: ...

class DefaultRetryExceededBehaviour(_message.Message):
    __slots__ = ("action", "variable", "variableType", "variableValue", "errorCode")
    ACTION_FIELD_NUMBER: _ClassVar[int]
    VARIABLE_FIELD_NUMBER: _ClassVar[int]
    VARIABLETYPE_FIELD_NUMBER: _ClassVar[int]
    VARIABLEVALUE_FIELD_NUMBER: _ClassVar[int]
    ERRORCODE_FIELD_NUMBER: _ClassVar[int]
    action: RetryErrorAction
    variable: str
    variableType: str
    variableValue: str
    errorCode: str
    def __init__(self, action: _Optional[_Union[RetryErrorAction, str]] = ..., variable: _Optional[str] = ..., variableType: _Optional[str] = ..., variableValue: _Optional[str] = ..., errorCode: _Optional[str] = ...) -> None: ...

class ParameterGroup(_message.Message):
    __slots__ = ("name", "short", "description")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SHORT_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    name: str
    short: str
    description: str
    def __init__(self, name: _Optional[str] = ..., short: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class Parameter(_message.Message):
    __slots__ = ("name", "short", "description", "type", "customTypeExtension", "collection", "group", "extensionData", "mandatory", "validateExpr", "example")
    class ExtensionDataEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    NAME_FIELD_NUMBER: _ClassVar[int]
    SHORT_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    CUSTOMTYPEEXTENSION_FIELD_NUMBER: _ClassVar[int]
    COLLECTION_FIELD_NUMBER: _ClassVar[int]
    GROUP_FIELD_NUMBER: _ClassVar[int]
    EXTENSIONDATA_FIELD_NUMBER: _ClassVar[int]
    MANDATORY_FIELD_NUMBER: _ClassVar[int]
    VALIDATEEXPR_FIELD_NUMBER: _ClassVar[int]
    EXAMPLE_FIELD_NUMBER: _ClassVar[int]
    name: str
    short: str
    description: str
    type: str
    customTypeExtension: str
    collection: bool
    group: str
    extensionData: _containers.ScalarMap[str, str]
    mandatory: bool
    validateExpr: str
    example: str
    def __init__(self, name: _Optional[str] = ..., short: _Optional[str] = ..., description: _Optional[str] = ..., type: _Optional[str] = ..., customTypeExtension: _Optional[str] = ..., collection: bool = ..., group: _Optional[str] = ..., extensionData: _Optional[_Mapping[str, str]] = ..., mandatory: bool = ..., validateExpr: _Optional[str] = ..., example: _Optional[str] = ...) -> None: ...

class Message(_message.Message):
    __slots__ = ("name", "correlationKey", "short", "description")
    NAME_FIELD_NUMBER: _ClassVar[int]
    CORRELATIONKEY_FIELD_NUMBER: _ClassVar[int]
    SHORT_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    name: str
    correlationKey: str
    short: str
    description: str
    def __init__(self, name: _Optional[str] = ..., correlationKey: _Optional[str] = ..., short: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class TaskError(_message.Message):
    __slots__ = ("name", "code", "short", "description")
    NAME_FIELD_NUMBER: _ClassVar[int]
    CODE_FIELD_NUMBER: _ClassVar[int]
    SHORT_FIELD_NUMBER: _ClassVar[int]
    DESCRIPTION_FIELD_NUMBER: _ClassVar[int]
    name: str
    code: str
    short: str
    description: str
    def __init__(self, name: _Optional[str] = ..., code: _Optional[str] = ..., short: _Optional[str] = ..., description: _Optional[str] = ...) -> None: ...

class RegisterTaskRequest(_message.Message):
    __slots__ = ("spec",)
    SPEC_FIELD_NUMBER: _ClassVar[int]
    spec: TaskSpec
    def __init__(self, spec: _Optional[_Union[TaskSpec, _Mapping]] = ...) -> None: ...

class RegisterTaskResponse(_message.Message):
    __slots__ = ("uid",)
    UID_FIELD_NUMBER: _ClassVar[int]
    uid: str
    def __init__(self, uid: _Optional[str] = ...) -> None: ...

class GetTaskSpecRequest(_message.Message):
    __slots__ = ("uid",)
    UID_FIELD_NUMBER: _ClassVar[int]
    uid: str
    def __init__(self, uid: _Optional[str] = ...) -> None: ...

class GetTaskSpecResponse(_message.Message):
    __slots__ = ("spec",)
    SPEC_FIELD_NUMBER: _ClassVar[int]
    spec: TaskSpec
    def __init__(self, spec: _Optional[_Union[TaskSpec, _Mapping]] = ...) -> None: ...

class DeprecateServiceTaskRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeprecateServiceTaskResponse(_message.Message):
    __slots__ = ("usage", "success")
    USAGE_FIELD_NUMBER: _ClassVar[int]
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    usage: TaskSpecUsageReport
    success: bool
    def __init__(self, usage: _Optional[_Union[TaskSpecUsageReport, _Mapping]] = ..., success: bool = ...) -> None: ...

class GetTaskSpecVersionsRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class GetTaskSpecVersionsResponse(_message.Message):
    __slots__ = ("versions",)
    VERSIONS_FIELD_NUMBER: _ClassVar[int]
    versions: TaskSpecVersions
    def __init__(self, versions: _Optional[_Union[TaskSpecVersions, _Mapping]] = ...) -> None: ...

class GetTaskSpecUsageRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class TaskSpecUsageReport(_message.Message):
    __slots__ = ("workflow", "process", "executingWorkflow", "executingProcessInstance")
    WORKFLOW_FIELD_NUMBER: _ClassVar[int]
    PROCESS_FIELD_NUMBER: _ClassVar[int]
    EXECUTINGWORKFLOW_FIELD_NUMBER: _ClassVar[int]
    EXECUTINGPROCESSINSTANCE_FIELD_NUMBER: _ClassVar[int]
    workflow: _containers.RepeatedScalarFieldContainer[str]
    process: _containers.RepeatedScalarFieldContainer[str]
    executingWorkflow: _containers.RepeatedScalarFieldContainer[str]
    executingProcessInstance: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, workflow: _Optional[_Iterable[str]] = ..., process: _Optional[_Iterable[str]] = ..., executingWorkflow: _Optional[_Iterable[str]] = ..., executingProcessInstance: _Optional[_Iterable[str]] = ...) -> None: ...

class ListTaskSpecUIDsRequest(_message.Message):
    __slots__ = ("IncludeDeprecated",)
    INCLUDEDEPRECATED_FIELD_NUMBER: _ClassVar[int]
    IncludeDeprecated: bool
    def __init__(self, IncludeDeprecated: bool = ...) -> None: ...

class ListTaskSpecUIDsResponse(_message.Message):
    __slots__ = ("uid",)
    UID_FIELD_NUMBER: _ClassVar[int]
    uid: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, uid: _Optional[_Iterable[str]] = ...) -> None: ...

class Sender(_message.Message):
    __slots__ = ("vars", "correlationKey")
    VARS_FIELD_NUMBER: _ClassVar[int]
    CORRELATIONKEY_FIELD_NUMBER: _ClassVar[int]
    vars: bytes
    correlationKey: str
    def __init__(self, vars: _Optional[bytes] = ..., correlationKey: _Optional[str] = ...) -> None: ...

class Receiver(_message.Message):
    __slots__ = ("id", "correlationKey")
    ID_FIELD_NUMBER: _ClassVar[int]
    CORRELATIONKEY_FIELD_NUMBER: _ClassVar[int]
    id: str
    correlationKey: str
    def __init__(self, id: _Optional[str] = ..., correlationKey: _Optional[str] = ...) -> None: ...

class Exchange(_message.Message):
    __slots__ = ("sender", "receivers")
    class ReceiversEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: Receiver
        def __init__(self, key: _Optional[str] = ..., value: _Optional[_Union[Receiver, _Mapping]] = ...) -> None: ...
    SENDER_FIELD_NUMBER: _ClassVar[int]
    RECEIVERS_FIELD_NUMBER: _ClassVar[int]
    sender: Sender
    receivers: _containers.MessageMap[str, Receiver]
    def __init__(self, sender: _Optional[_Union[Sender, _Mapping]] = ..., receivers: _Optional[_Mapping[str, Receiver]] = ...) -> None: ...

class TelemetryClients(_message.Message):
    __slots__ = ("count",)
    COUNT_FIELD_NUMBER: _ClassVar[int]
    count: int
    def __init__(self, count: _Optional[int] = ...) -> None: ...

class LogRequest(_message.Message):
    __slots__ = ("hostname", "clientId", "trackingId", "level", "time", "source", "message", "attributes")
    class AttributesEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    CLIENTID_FIELD_NUMBER: _ClassVar[int]
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    LEVEL_FIELD_NUMBER: _ClassVar[int]
    TIME_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    ATTRIBUTES_FIELD_NUMBER: _ClassVar[int]
    hostname: str
    clientId: str
    trackingId: bytes
    level: int
    time: int
    source: LogSource
    message: str
    attributes: _containers.ScalarMap[str, str]
    def __init__(self, hostname: _Optional[str] = ..., clientId: _Optional[str] = ..., trackingId: _Optional[bytes] = ..., level: _Optional[int] = ..., time: _Optional[int] = ..., source: _Optional[_Union[LogSource, str]] = ..., message: _Optional[str] = ..., attributes: _Optional[_Mapping[str, str]] = ...) -> None: ...

class LogResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetCompensationInputVariablesRequest(_message.Message):
    __slots__ = ("processInstanceId", "trackingId")
    PROCESSINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    processInstanceId: str
    trackingId: str
    def __init__(self, processInstanceId: _Optional[str] = ..., trackingId: _Optional[str] = ...) -> None: ...

class GetCompensationInputVariablesResponse(_message.Message):
    __slots__ = ("vars",)
    VARS_FIELD_NUMBER: _ClassVar[int]
    vars: bytes
    def __init__(self, vars: _Optional[bytes] = ...) -> None: ...

class GetCompensationOutputVariablesRequest(_message.Message):
    __slots__ = ("processInstanceId", "trackingId")
    PROCESSINSTANCEID_FIELD_NUMBER: _ClassVar[int]
    TRACKINGID_FIELD_NUMBER: _ClassVar[int]
    processInstanceId: str
    trackingId: str
    def __init__(self, processInstanceId: _Optional[str] = ..., trackingId: _Optional[str] = ...) -> None: ...

class GetCompensationOutputVariablesResponse(_message.Message):
    __slots__ = ("vars",)
    VARS_FIELD_NUMBER: _ClassVar[int]
    vars: bytes
    def __init__(self, vars: _Optional[bytes] = ...) -> None: ...

class FatalError(_message.Message):
    __slots__ = ("handlingStrategy", "workflowState")
    HANDLINGSTRATEGY_FIELD_NUMBER: _ClassVar[int]
    WORKFLOWSTATE_FIELD_NUMBER: _ClassVar[int]
    handlingStrategy: HandlingStrategy
    workflowState: WorkflowState
    def __init__(self, handlingStrategy: _Optional[_Union[HandlingStrategy, str]] = ..., workflowState: _Optional[_Union[WorkflowState, _Mapping]] = ...) -> None: ...
