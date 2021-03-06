localCI
=======

LocalCI is a Continuous Integration System designed to be used in two ways

- As a service - testing the master branch and pull requests of the repositories
- As a tool - testing a specific pull request of a repository

**localCI** defines 5 stages when a repository is being tested

- ``Setup`` - The commands needed to setup the environment
- ``Run`` - The commands needed to run the test
- ``Teardown`` - The commands needed to cleanup the environment
- ``On Success`` - The commands to be executed if ``Setup`` , ``Run`` and ``Teardown`` finished correctly
- ``On Failure`` - The commands to be executed if any of ``Setup`` , ``Run`` or ``Teardown`` fail.

The commands of each stage run sequentially, inherit the environment variables of the system and other
`Environment variables`_ are exported.

Logging
-------

By default **localCI** places all the logs generated by each stage in ``/var/log/localCI/`` inside a directory
with the same number as the pull request or the master branch SHA. To specify a different path, modify
``logDir`` in the `Configuration file`_. One log file per stage is generated with the same name.
For example, if the ``logDir`` is ``/var/log/localCI`` and the pull request being tested is the number ``53``,
when the stage ``setup`` is run the full path to its log file would be ``/var/log/localCI/53/setup``.
Alternatively, if the ``logDir`` is ``/var/log/localCI/runtime`` and the master branch is being tested,
when the stage ``run`` is run, the full path to its log file would depend on the master branch SHA and
be similar to ``/var/log/localCI/runtime/1cefbb994cd6cc1341473bccd2284cd6cc1398abc3d9a50459fc808df550/run``.


Environment variables
---------------------

Before running the commands of each stage, **localCI** exports the following environment variables:

+---------------------+----------------------+-----------------------------------------------------+
| Name                | Value                | Description                                         |
+=====================+======================+=====================================================+
| CI                  | true                 | Running in Continuous Integration System            |
+---------------------+----------------------+-----------------------------------------------------+
| LOCALCI             | true                 | The CI is **localCI**                               |
+---------------------+----------------------+-----------------------------------------------------+
| LOCALCI_REPO_SLUG   | E.g foo/bar          | Repository owner / repository name                  |
+---------------------+----------------------+-----------------------------------------------------+
| LOCALCI_BRANCH_NAME | E.g master           | The name of the branch                              |
+---------------------+----------------------+-----------------------------------------------------+

Variables available only for pull requests:

+---------------------+----------------------+-----------------------------------------------------+
| Name                | Value                | Description                                         |
+=====================+======================+=====================================================+
| LOCALCI_PR_NUMBER   | E.g 23               | Pull request number                                 |
+---------------------+----------------------+-----------------------------------------------------+


Configuration file
------------------
The configuration file follows the syntax defined by the `Tom's Obvious, Minimal Language`_ and can contain next keys and sections:

Keys
~~~~

- ``runTestsInParallel [bool] (optional)``: specify whether the tests can run in parallel. By default this value is ``false``. If **localCI** is running as a tool the value is ignored.

Sections
~~~~~~~~

``[[Repo]]``
............
Contains information about a repository and can appear more than once.
This section have next keys:

- ``url [string] (mandatory)``: the URL to the repository.
- ``masterBranch [string] (optional)``: **localCI** will test this branch each time a change is merged in this branch. By default this value is ``master``.
- ``pr [int] (optional)``: the pull request to test. If **localCI** is running as a service then only this pull request is monitored and tested.
- ``refreshTime [string] (optional)``: specify the time to wait for checking if a pull request needs to be tested. This option is used only when
  **localCI** runs as a service.  By default this value is 30s. For more information about formats and suffixes supported see `ParseDuration`_
- ``token [string] (optional)``: the repository access token. If the control version repository imposes a rate limit,
  this token is used to make the requests and post in the pull request the messages specified in ``postOnFailure``
  and ``postOnSuccess``.
- ``setup [[]string] (optional)``: the commands needed to setup the test environment.
- ``run [[]string] (mandatory)``: the commands used to test the pull request once ``setup`` finished correctly.
- ``teardown [[]string] (optional)``: the commands executed to cleanup the environment doesn't matter if ``setup`` and ``run`` finished correctly or not.
- ``onSuccess [[]string] (optional)``: the commands to be executed if ``setup``, ``run`` and ``teardown`` finished correctly.
- ``onfailure [[]string] (optional)``: the commands to be executed if any of ``setup``, ``run`` or ``teardown`` fail.
- ``postOnSuccess [string] (optional)``: message to be posted in the pull request if ``setup``, ``run`` and ``teardown`` finished correctly.
  If this key is not present or is an empty string no mesage is posted.
- ``postOnFailure [string] (optional)``: message to be posted in the pull request if any of ``setup``, ``run`` or ``teardown`` fail.
  If this key is not present or is an empty string no mesage is posted.
- ``logDir [string] (optional)``: the directory where the logs are placed. By default this value is ``/var/log/localCI``
- ``whitelist [string] (optional)``: a comma-separated list of the users whose pull requests can be tested. This key has two wildcards ``*`` and ``@``.
  ``*`` means that the pull request of any user can be tested and ``@`` means that the pull request of any user belonging to the organization can be tested, any
  other word or character is interpreted as an user name. By default this value is ``*``.

and have next sub-sections

- ``[Repo.language] (mandatory)``

  The language of the project.

  - ``language [string] (mandatory)``: the language of the project. To check the list of supported languages see `Supported languages`_
  - ``version [string] (madatory if 'language = Go')``: the version of the ``language`` that must be used to test the project.

- ``[Repo.commentTrigger] (optional)``

  Comment needded in the pull request to test it.
  If new commits were pushed after the comment the pull request will not be tested until the same comment is added again.

  - ``user [string] (optional)``: the user that must write the comment to test the pull request.
  - ``comment [string] (mandatory)``: comment needed to test the pull request.

- ``[Repo.logServer] (optional)``

  The server where the logs of each test are copied.

  - ``ip [string] (mandatory)``: the server IP.
  - ``user [string] (optional)``: the server user. By default this value is ``root``.
  - ``dir [string] (optional)``: the server directory where the logs are copied. By default this values is ``/var/log/localCI``.
  - ``key [string] (optional)``: the private key to login into the server.


Configuration file example
..........................
::

   runTestsInParallel = true

   [[Repo]]
   url = "https://github.com/clearcontainers/runtime"
   masterBranch = "master"
   pr = 563
   refreshTime = "60s"
   token = "YOUR ACCESS TOKEN"
   setup = [ ".ci/setup.sh" ]
   run = [ ".ci/run.sh" ]
   teardown = [ ".ci/teardown.sh" ]
   onSuccess = [ "echo success" ]
   onfailure = [ "echo failure" ]
   postOnSuccess = "qa-passed"
   postOnFailure = "qa-failed"
   logDir = "/var/log/localCI"
   whitelist = "@"
   [Repo.language]
     language = "Go"
     version = "go1.8.3"
   [Repo.commentTrigger]
     user = "QA-bot"
     comment = "qa-passed"
   [Repo.logServer]
	 ip = "192.168.1.15"
	 user = "USER"
	 dir = "/var/log/localCI"
	 key = """
	 -----BEGIN OPENSSH PRIVATE KEY-----
	 YOUR PRIVATE SSH KEY
	 -----END OPENSSH PRIVATE KEY-----
	 """

Supported languages
-------------------
- ``Go`` - The Go programming language. An example of a valid version is ``go1.8.3``. To check all possible versions see `Go Downloads`_


Tests
-----

To run the basic unit tests, run::

  $ make check


.. _`Tom's Obvious, Minimal Language`: https://github.com/toml-lang/toml
.. _`ParseDuration`: https://golang.org/pkg/time/#ParseDuration
.. _`Go Downloads`: https://golang.org/dl/
