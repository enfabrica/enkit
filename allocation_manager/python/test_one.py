from typing import List, Union
from enum import Enum, auto

class State(Enum):
    FREE = auto()
    RESERVED = auto()


class Resource:
    def __init__(self, name: str = '') -> None:
        self.state = State.FREE
        self.name = name or 'randomize name her'
        self.child = []
    
    def having(self, cls, count: int = 1):
        for i in range(count):
            self.child.append(cls())

        return self


class Machine(Resource):
    def __init__(self, name: str) -> None:
        super().__init__(name)


class CPU(Resource):
    """cpu resource
    """
    def __init__(self) -> None:
        """cpu resource

        TODO: named cpu for the case when CPUs are assigned and not shared
        """
        super().__init__()


class Pool:
    def __init__(self, data: List[Resource] = None) -> None:
        self.pool = data or []

    def append(self, r: Resource) -> None:
        self.pool.append(r)
        raise NotImplemented('no usage found')

    def extend(self, l: List[Resource]) -> 'Pool':
        self.pool.extend(l)

        return self

    def len(self) -> int:
        return len(self.pool)

    def __iter__(self):
        raise NotImplementedError
        # return iter(self.pool)
    
    def __contains__(self, item):
        for each in self.pool:
            if each.state == State.FREE and isinstance(each, type(item)):
                return True

        return False

    def allocate(self, kind_of) -> Resource:
        for each in self.pool:
            if each.state == State.FREE and isinstance(each, kind_of):
                each.state = State.RESERVED
                return each

        return None


def test_simple_resource_abc():
    pool = Pool().extend([Resource(), Resource()])

    assert pool.len() == 2

    assert Resource() in pool
    
    a = pool.allocate(Resource)
    assert isinstance(a, Resource)
    print(a)

    b = pool.allocate(Resource)
    assert isinstance(b, Resource)
    print(b)

    c = pool.allocate(Resource)
    assert c is None


def test_machine():
    pool = Pool([Resource(), Machine('server2'), Machine('server1')])

    a = pool.allocate(Resource)
    assert isinstance(a, Resource)
    assert not isinstance(a, Machine)
    print(a)

    b = pool.allocate(Machine)
    assert isinstance(b, Machine)
    print(b)

    c = pool.allocate(Machine)
    assert isinstance(c, Machine)
    print(c)

    d = pool.allocate(Machine)
    assert d is None


def test_machine_with_cpu():
    pool = Pool([Resource(),
                 Machine('server2').having(CPU, 4),
                 Machine('server1')])

    a = pool.allocate(Resource)
    assert isinstance(a, Resource)
    assert not isinstance(a, Machine)
    assert len(a.child) in [0, 4]
    print(a)

    b = pool.allocate(Machine)
    assert isinstance(b, Machine)
    assert len(b.child) in [0, 4]
    print(b)

    c = pool.allocate(Machine)
    assert isinstance(c, Machine)
    print(c)

    d = pool.allocate(Machine)
    assert d is None
