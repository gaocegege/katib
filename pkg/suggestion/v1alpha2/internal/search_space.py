import logging
from pkg.api.v1alpha2.python import api_pb2

MAX_GOAL = "MAXIMIZE"
MIN_GOAL = "MINIMIZE"

INTEGER = "INTEGER"
DOUBLE = "DOUBLE"
CATEGORICAL = "CATEGORICAL"
DISCRETE = "DISCRETE"

logging.basicConfig(level=logging.DEBUG)
logger = logging.getLogger("HyperParameterSearchSpace")


class HyperParameterSearchSpace(object):
    def __init__(self):
        self.goal = ""
        self.params = []

    @staticmethod
    def convert(experiment):
        search_space = HyperParameterSearchSpace()
        if experiment.spec.objective.type == api_pb2.MAXIMIZE:
            search_space.goal = MAX_GOAL
        elif experiment.spec.objective.type == api_pb2.MINIMIZE:
            search_space.goal = MIN_GOAL
        for p in experiment.spec.parameter_specs.parameters:
            search_space.params.append(
                HyperParameterSearchSpace.convertParameter(p))
        return search_space

    def __str__(self):
        return "HyperParameterSearchSpace(goal: {}, ".format(self.goal) + \
            "params: {})".format(", ".join([element.__str__() for element in self.params]))

    @staticmethod
    def convertParameter(p):
        if p.parameter_type == api_pb2.INT:
            return HyperParameter.int(p.name, p.feasible_space.min, p.feasible_space.max)
        elif p.parameter_type == api_pb2.DOUBLE:
            return HyperParameter.double(p.name, p.feasible_space.min, p.feasible_space.max)
        elif p.parameter_type == api_pb2.CATEGORICAL:
            return HyperParameter.categorical(p.name, p.feasible_space.list)
        elif p.parameter_type == api_pb2.DISCRETE:
            return HyperParameter.discrete(p.name, p.feasible_space.list)
        else:
            logger.error(
                "Cannot get the type for the parameter: %s (%s)", p.name, p.parameter_type)


class HyperParameter(object):
    def __init__(self, name, type, min, max, list):
        self.name = name
        self.type = type
        self.min = min
        self.max = max
        self.list = list

    def __str__(self):
        if self.type == INTEGER or self.type == DOUBLE:
            return "HyperParameter(name: {}, type: {}, min: {}, max: {})".format(
                self.name, self.type, self.min, self.max)
        else:
            return "HyperParameter(name: {}, type: {}, list: {})".format(
                self.name, self.type, ", ".join(self.list))

    @staticmethod
    def int(name, min, max):
        return HyperParameter(name, INTEGER, min, max, [])

    @staticmethod
    def double(name, min, max):
        return HyperParameter(name, DOUBLE, min, max, [])

    @staticmethod
    def categorical(name, lst):
        return HyperParameter(name, CATEGORICAL, 0, 0, [str(e) for e in lst])

    @staticmethod
    def discrete(name, lst):
        return HyperParameter(name, DISCRETE, 0, 0, [str(e) for e in lst])