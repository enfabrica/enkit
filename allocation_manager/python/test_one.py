from typing import List
from enum import Enum, auto

class State(Enum):
    FREE = auto()
    RESERVED = auto()


class Resource:
    def __init__(self, name: str = '') -> None:
        self.state = State.FREE
        self.name = name or 'randomize name her'


class Pool:
    def __init__(self) -> None:
        self.pool = []

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

    def reserve(self, kindof) -> Resource:
        for each in self.pool:
            if each.state == State.FREE and isinstance(each, kindof):
                each.state = State.RESERVED
                return each

        return None


def test_one():
    pool = Pool().extend([Resource(), Resource()])

    assert pool.len() == 2

    assert Resource() in pool
    
    a = pool.reserve(Resource)
    assert isinstance(a, Resource)
    print(a)

    b = pool.reserve(Resource)
    assert isinstance(b, Resource)
    print(b)

    c = pool.reserve(Resource)
    assert c is None
